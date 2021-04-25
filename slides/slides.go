package slides

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/present"
)

// Config specifies Server configuration values.
type Config struct {
	ContentPath  string // Relative or absolute location of article files and related content.
	TemplatePath string // Relative or absolute location of template files.
	BasePath     string // Base URL path relative to server root (no trailing slash).
	PlayEnabled  bool
}

// containsSpecialFile reports whether name contains a path element starting with a period
// or is another kind of special file. The name is assumed to be a delimited by forward
// slashes, as guaranteed by the http.FileSystem interface.
func containsSpecialFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func NewServer(conf Config) (http.Handler, error) {
	fs := http.FileServer(http.Dir(conf.ContentPath))
	return http.StripPrefix(conf.BasePath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if containsSpecialFile(r.URL.Path) {
			log.Println("Path not allowed")
			http.Error(w, "Path not allowed", http.StatusBadRequest)
			return
		}
		name := filepath.Join(".", filepath.FromSlash(r.URL.Path))
		if isDoc(name) {
			err := conf.renderDoc(w, conf.ContentPath, name)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), 500)
			}
			return
		}
		// http.FileServer(http.Dir(conf.ContentPath)).ServeHTTP(w, r)
		fs.ServeHTTP(w, r)
	})), nil
}

// extensions maps the presentable file extensions to the name of the
// template to be executed.
var extensions = map[string]string{
	".slide":   "slides.tmpl",
	".article": "article.tmpl",
}

func isDoc(path string) bool {
	_, ok := extensions[filepath.Ext(path)]
	return ok
}

func (conf Config) playable(c present.Code) bool {
	//log.Print(present.PlayEnabled, c.Play, conf.PlayEnabled, c.Ext)
	return present.PlayEnabled && c.Play && conf.PlayEnabled && c.Ext == ".go"
}

// renderDoc reads the present file, builds its template representation,
// and executes the template, sending output to w.
func (conf Config) renderDoc(w io.Writer, base, docFile string) error {
	// Read the input and build the doc structure.
	doc, err := parse(filepath.Join(base, docFile), 0)
	if err != nil {
		return err
	}

	// Find which template should be executed.
	ext := filepath.Ext(docFile)
	contentTmpl, ok := extensions[ext]
	if !ok {
		return fmt.Errorf("no template for extension %v", ext)
	}

	// Locate the template file.
	actionTmpl := filepath.Join(conf.TemplatePath, "action.tmpl")
	contentTmpl = filepath.Join(conf.TemplatePath, contentTmpl)

	// Read and parse the input.
	tmpl := present.Template()
	tmpl = tmpl.Funcs(template.FuncMap{"playable": func(c present.Code) bool { return conf.playable(c) }})
	if _, err := tmpl.ParseFiles(actionTmpl, contentTmpl); err != nil {
		return err
	}

	// Execute the template.
	return doc.Render(w, tmpl)
}

func parse(name string, mode present.ParseMode) (*present.Doc, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return present.Parse(f, name, 0)
}

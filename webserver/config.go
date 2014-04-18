package webserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LocateConfigFile determines where the config file should be found based on the location of the current executable
func LocateConfigFile(basename string) string {
	var exePath, err = exec.LookPath(os.Args[0])
	if err != nil {
		log.Print("Warning: ", err)
		exePath = os.Args[0]
	}
	s, err := filepath.Abs(exePath)
	if err != nil {
		log.Print("Warning: ", err)
	} else {
		exePath = s
	}
	exePath, _ = filepath.Split(exePath)
	exePath = filepath.ToSlash(exePath)

	if strings.HasPrefix(exePath, "/usr/bin/") {
		exePath = strings.Replace(exePath, "/usr/bin/", "/etc/", 1)
	} else {
		exePath = strings.Replace(exePath, "/bin/", "/etc/", 1)
	}

	return filepath.FromSlash(exePath) + basename
}

// Server respresents a given web server serving on a particular port
type Server struct {
	Addr  string `json:"Addr"`  // address to serve on
	Hosts []Host `json:"Hosts"` // hosts at this address
}

// Host represents data about the things to server on a given hostname
type Host struct {
	Hostname     string `json:"Hostname"`     // name of host
	VDirs        []VDir `json:"VDirs"`        // virtual directories for serving static files
	Blogs        []Blog `json:"Blogs"`        // virtual directories for serving dynamic blogs
	PlayEnabled  bool   `json:"PlayEnabled"`  // whether running go code from the browser is enabled
	NativeClient bool   `json:"NativeClient"` // whether to use a native client when running go codde
}

// VDir represents a virtual directory for serving static files
type VDir struct {
	Root   string `json:"Root"`   // Root of the vdir on the web server
	Folder string `json:"Folder"` // Folder on disk
}

// Blog represents a blog to be served
type Blog struct {
	Root         string `json:"Root"`         // Root of the blog on the web server
	Folder       string `json:"Folder"`       // Folder on disk
	HomeArticles int    `json:"HomeArticles"` // How many articles to show on the home page
	FeedArticles int    `json:"FeedArticles"` // How many articles to show in the atom feed
	FeedTitle    string `json:"FeedTitle"`    // Title of the atom feed
}

// Config holds the configuration of the web server
type Config struct {
	Servers []Server `json:"Servers"` // List of servers (addresses) we are serving
	Reload  bool     // Whether to reload blogs on every visit (slow - don't use in production. For local editing.)
	Log     bool     // Whether to log requests
}

// ReadConfig reads the configuration file at the given location
func ReadConfig(cfgFile string) (c Config) {
	f, err := os.Open(cfgFile)
	if err != nil {
		log.Fatalf("Unable to open configuration file %s: %s", cfgFile, err)
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("Unable to read configuration file %s: %s", cfgFile, err)
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		log.Fatalf("Unable to parse configuration file %s: %s", cfgFile, err)
	}
	for _, s := range c.Servers {
		if len(s.Hosts) == 0 {
			log.Fatalf("No Hosts specified in configuration file %s for server %s", cfgFile, s.Addr)
		}
		for _, h := range s.Hosts {
			h.Hostname = strings.TrimSpace(h.Hostname)
			if h.Hostname == "" {
				log.Fatalf("Invalid Hostname specified in configuration file %s: \"%s\"", cfgFile, h.Hostname)
			}
			for _, v := range h.VDirs {
				v.Folder = strings.TrimSpace(v.Folder)
				if v.Folder == "" {
					log.Fatalf("Invalid vdir Folder specified in configuration file %s for host \"%s\": \"%s\"", cfgFile, h.Hostname, v.Folder)
				}
				_, err := os.Stat(v.Folder)
				if err != nil {
					log.Printf("Warning: Cannot stat folder \"%s\": %s", v.Folder, err)
				}
				v.Root = strings.TrimSpace(v.Root)
				if v.Root == "" {
					log.Fatalf("Invalid vdir Root specified in configuration file %s for host \"%s\": \"%s\"", cfgFile, h.Hostname, v.Root)
				}
			}
			for _, v := range h.Blogs {
				v.Folder = strings.TrimSpace(v.Folder)
				if v.Folder == "" {
					log.Fatalf("Invalid blog Folder specified in configuration file %s for host \"%s\": \"%s\"", cfgFile, h.Hostname, v.Folder)
				}
				_, err := os.Stat(v.Folder)
				if err != nil {
					log.Printf("Warning: Cannot stat folder \"%s\": %s", v.Folder, err)
				}
				v.Root = strings.TrimSpace(v.Root)
				if v.Root == "" {
					log.Fatalf("Invalid blog Root specified in configuration file %s for host \"%s\": \"%s\"", cfgFile, h.Hostname, v.Root)
				}
				if v.HomeArticles <= 0 {
					log.Fatalf("Invalid blog HomeArticles specified in configuration file %s for host \"%s\": %d", cfgFile, h.Hostname, v.HomeArticles)
				}
				if v.FeedArticles <= 0 {
					log.Fatalf("Invalid blog FeedArticles specified in configuration file %s for host \"%s\": %d", cfgFile, h.Hostname, v.FeedArticles)
				}
			}
			if len(h.VDirs) == 0 && len(h.Blogs) == 0 {
				log.Fatalf("No VDirs or Blogs specified in configuration file %s for host \"%s\"", cfgFile, h.Hostname)
			}
		}
		s.Addr = strings.TrimSpace(s.Addr)
		if s.Addr == "" {
			s.Addr = ":8080"
		}
	}
	return
}

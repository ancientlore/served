package webserver

import (
	"github.com/ancientlore/served/slides"
	"golang.org/x/tools/blog"
	"golang.org/x/tools/godoc/static"
	_ "golang.org/x/tools/playground"
	"golang.org/x/tools/playground/socket"
	"golang.org/x/tools/present"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CreateServers creates the handlers needed for serving the site(s) listed in the given configuration.
func CreateServers(conf Config) (map[string]http.Handler, error) {
	handlers := make(map[string]http.Handler, 0)
	play := false
	for _, s := range conf.Servers {
		hostMap := make(map[string]*http.ServeMux, 0)
		for _, host := range s.Hosts {
			mux := http.NewServeMux()
			if host.PlayEnabled {
				play = true
			}
			for _, v := range host.Blogs {
				if !v.Disabled {
					config := blog.Config{
						Hostname:     host.Hostname,
						BaseURL:      "http://" + host.Hostname,
						BasePath:     strings.TrimSuffix(v.Root, "/"),
						GodocURL:     "",
						HomeArticles: v.HomeArticles, // articles to display on the home page
						FeedArticles: v.FeedArticles, // articles to include in Atom and JSON feeds
						PlayEnabled:  host.PlayEnabled,
						FeedTitle:    v.FeedTitle,
						ContentPath:  filepath.Join(v.Folder, "content"),
						TemplatePath: filepath.Join(v.Folder, "template"),
					}
					var h http.Handler
					var err error
					if conf.Reload {
						h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							s, err := blog.NewServer(config)
							if err != nil {
								http.Error(w, err.Error(), 500)
								return
							}
							s.ServeHTTP(w, r)
						})
					} else {
						h, err = blog.NewServer(config)
						if err != nil {
							return nil, err
						}
					}
					if conf.Log {
						h = logRequest(h)
					}
					mux.Handle(v.Root, h)

					// slide server
					slidePath := strings.TrimSuffix(v.Root, "/") + "/slides/"
					sconf := slides.Config{
						ContentPath:  filepath.Join(v.Folder, "content"),
						TemplatePath: filepath.Join(v.Folder, "template/slides"),
						BasePath:     slidePath,
						PlayEnabled:  host.PlayEnabled,
					}
					h, err = slides.NewServer(sconf)
					if err != nil {
						return nil, err
					}
					if conf.Log {
						h = logRequest(h)
					}
					mux.Handle(slidePath, h)
				} else {
					log.Printf("Skipping disabled blog %s%s", host.Hostname, v.Root)
				}
			}

			if len(host.Blogs) > 0 {
				mux.Handle("/lib/godoc/", http.StripPrefix("/lib/godoc/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					name := r.URL.Path
					b, ok := static.Files[name]
					if !ok {
						http.NotFound(w, r)
						return
					}
					http.ServeContent(w, r, name, time.Time{}, strings.NewReader(b))
				})))

				if host.PlayEnabled {
					if host.NativeClient {
						socket.RunScripts = false
						socket.Environ = func() []string {
							if runtime.GOARCH == "amd64" {
								return environ("GOOS=nacl", "GOARCH=amd64p32")
							}
							return environ("GOOS=nacl")
						}
					}
					// playScript(basePath, "SocketTransport")
					origin := &url.URL{Scheme: "http"}
					h, p, err := net.SplitHostPort(s.Addr)
					if err != nil {
						log.Printf("Cannot enable play on %s: %s", host.Hostname, err)
					} else {
						if host.Hostname != "" {
							h = host.Hostname
						}
						if p == "" {
							p = "80"
						}
						origin.Host = net.JoinHostPort(h, p)
						mux.Handle("/socket", socket.NewHandler(origin))

						if !strings.HasPrefix(host.Hostname, "127.0.0.1") &&
							!strings.HasPrefix(host.Hostname, "localhost") &&
							host.PlayEnabled && !host.NativeClient {
							log.Print(localhostWarning)
						}
					}
				}
			}

			for _, v := range host.VDirs {
				if !v.Disabled {
					var h http.Handler
					h = http.FileServer(http.Dir(v.Folder))
					if v.Root != "/" {
						h = http.StripPrefix(v.Root, h)
					}
					if conf.Log {
						h = logRequest(h)
					}
					mux.Handle(v.Root, h)
				} else {
					log.Printf("Skipping disabled vdir %s%s", host.Hostname, v.Root)
				}
			}

			hostMap[host.Hostname] = mux
		}

		handlers[s.Addr] = selectHost(hostMap)
	}
	if play {
		present.PlayEnabled = true
	}

	return handlers, nil
}

// selectHost picks the server mux needed for the given host name
func selectHost(hostMap map[string]*http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts := strings.Split(r.Host, ":")
		host := ""
		if len(hosts) > 0 {
			host = hosts[0]
		}
		m, ok := hostMap[host]
		if !ok || m == nil {
			http.NotFound(w, r)
		} else {
			m.ServeHTTP(w, r)
		}
	})
}

// logRequest logs requests before serving them
func logRequest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Print(r.Host, r.URL)
		h.ServeHTTP(w, r)
	})
}

// environ is used to provide environment settings for go's playground feature
func environ(vars ...string) []string {
	env := os.Environ()
	for _, r := range vars {
		k := strings.SplitAfter(r, "=")[0]
		var found bool
		for i, v := range env {
			if strings.HasPrefix(v, k) {
				env[i] = r
				found = true
			}
		}
		if !found {
			env = append(env, r)
		}
	}
	return env
}

const localhostWarning = `
WARNING!  WARNING!  WARNING!

The present server appears to be listening on an address that is not localhost.
Anyone with access to this address and port will have access to this machine as
the user running present.

To avoid this message, listen on localhost or run with -play=false.

If you don't understand this message, hit Control-C to terminate this process.

WARNING!  WARNING!  WARNING!
`

// served is a simple web server for static files.
package main

import (
	"bitbucket.org/kardianos/service"
	"code.google.com/p/go.tools/blog"
	"code.google.com/p/go.tools/godoc/static"
	_ "code.google.com/p/go.tools/playground"
	"code.google.com/p/go.tools/playground/socket"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

const (
	VERSION = "0.2"
)

var (
	cpuprofile string
	memprofile string
	wsServed   service.Service
	svcRun     bool
	conf       Config
	cfgFile    string
	doLog      bool
	reload     bool
)

func locateConfigFile() string {
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

	return filepath.FromSlash(exePath) + "served.config"
}

type Server struct {
	Addr  string `json:"Addr"`
	Hosts []Host `json:"Hosts"`
}

type Host struct {
	Hostname     string `json:"Hostname"`
	VDirs        []VDir `json:"VDirs"`
	Blogs        []Blog `json:"Blogs"`
	PlayEnabled  bool   `json:"PlayEnabled"`
	NativeClient bool   `json:"NativeClient"`
}

type VDir struct {
	Root   string `json:"Root"`
	Folder string `json:"Folder"`
}

type Blog struct {
	Root         string `json:"Root"`
	Folder       string `json:"Folder"`
	HomeArticles int    `json:"HomeArticles"`
	FeedArticles int    `json:"FeedArticles"`
	FeedTitle    string `json:"FeedTitle"`
}

type Config struct {
	Servers []Server `json:"Servers"`
}

func readConfig(cfgFile string) (c Config) {
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
			if len(h.VDirs) == 0 {
				log.Fatalf("No VDirs specified in configuration file %s for host \"%s\"", cfgFile, h.Hostname)
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
		}
		s.Addr = strings.TrimSpace(s.Addr)
		if s.Addr == "" {
			s.Addr = ":8080"
		}
	}
	return
}

func SelectHost(hostMap map[string]*http.ServeMux) http.Handler {
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

func LogRequest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Print(r.Host, r.URL)
		h.ServeHTTP(w, r)
	})
}

func init() {
	const (
		name        = "served"
		displayName = "Simple Web Server"
		desc        = "served is a simple web server written in Go."
	)

	var help bool
	var ver bool
	var svcInstall bool
	var svcRemove bool
	var svcStart bool
	var svcStop bool

	defaultCfgFile := os.Getenv("SERVED_CONFIG")
	if defaultCfgFile == "" {
		defaultCfgFile = locateConfigFile()
	}

	flag.StringVar(&cfgFile, "config", defaultCfgFile, "Use to override the configuration file")
	flag.BoolVar(&help, "help", false, "Show command help")
	flag.BoolVar(&ver, "version", false, "Show version")
	flag.StringVar(&cpuprofile, "cpuprofile", "", "Write CPU profile to file")
	flag.StringVar(&memprofile, "memprofile", "", "Write memory profile to file")
	flag.BoolVar(&svcInstall, "install", false, "Install served as a service")
	flag.BoolVar(&svcRemove, "remove", false, "Remove served service")
	flag.BoolVar(&svcRun, "run", false, "Run served standalone (not as a service)")
	flag.BoolVar(&svcStart, "start", false, "Start served service")
	flag.BoolVar(&svcStop, "stop", false, "Stop served service")
	flag.BoolVar(&doLog, "log", false, "Log requests")
	flag.BoolVar(&reload, "reload", false, "reload blog content on each page load")

	flag.Parse()

	if help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if ver {
		fmt.Printf("served %s\n", VERSION)
		os.Exit(0)
	}

	// read config
	conf = readConfig(cfgFile)

	var err error
	wsServed, err = service.NewService(name, displayName, desc)
	if err != nil {
		log.Fatal(err)
	}

	if svcInstall == true && svcRemove == true {
		log.Fatalln("Options -install and -remove cannot be used together.")
	} else if svcInstall == true {
		err = wsServed.Install()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" installed.\n", displayName)
		os.Exit(0)
	} else if svcRemove == true {
		err = wsServed.Remove()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" removed.\n", displayName)
		os.Exit(0)
	} else if svcStart == true {
		err = wsServed.Start()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" started.\n", displayName)
		os.Exit(0)
	} else if svcStop == true {
		err = wsServed.Stop()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Service \"%s\" stopped.\n", displayName)
		os.Exit(0)
	}
}

func main() {
	var err error

	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if svcRun == true {
		startWork()
		sigChan := getSignalChan()
		for {
			select {
			case event := <-sigChan:
				log.Print(event)
				switch event {
				case os.Interrupt, os.Kill: //SIGINT, SIGKILL
					return
				}
			}
		}
		stopWork()
	} else {
		err = wsServed.Run(func() error {
			// start
			go startWork()
			wsServed.Info(fmt.Sprintf("Started served using config file \"%s\"", cfgFile))
			log.Printf("Started served using config file \"%s\"\n", cfgFile)
			return nil
		}, func() error {
			// stop
			stopWork()
			wsServed.Info("Stopped served")
			log.Println("Stopped served")
			return nil
		})
		if err != nil {
			wsServed.Error(err.Error())
			log.Println(err)
		}
	}
}

func startWork() {
	for _, s := range conf.Servers {
		hostMap := make(map[string]*http.ServeMux, 0)
		for _, host := range s.Hosts {
			mux := http.NewServeMux()

			for _, v := range host.Blogs {
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
				if reload {
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
						log.Fatal(err)
					}
				}
				if doLog {
					h = LogRequest(h)
				}
				mux.Handle(v.Root, h)
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
					mux.Handle("/socket", socket.Handler)
				}
			}

			for _, v := range host.VDirs {
				var h http.Handler
				h = http.FileServer(http.Dir(v.Folder))
				if v.Root != "/" {
					h = http.StripPrefix(v.Root, h)
				}
				if doLog {
					h = LogRequest(h)
				}
				mux.Handle(v.Root, h)
			}

			hostMap[host.Hostname] = mux
		}

		go func(addr string, hm map[string]*http.ServeMux) {
			if err := http.ListenAndServe(addr, SelectHost(hm)); err != nil {
				log.Fatal("ListenAndServe: ", err)
			}
		}(s.Addr, hostMap)
		log.Printf("Server %s added", s.Addr)
	}
}

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

func stopWork() {
	// write memory profile if configured
	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			log.Print(err)
		} else {
			pprof.WriteHeapProfile(f)
			f.Close()
		}
	}
}

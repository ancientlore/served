// served is a simple web server for static files.
package main

import (
	"bitbucket.org/kardianos/service"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strings"
)

const (
	VERSION = "0.1"
)

var (
	cpuprofile string
	memprofile string
	wsServed   service.Service
	svcRun     bool
	conf       Config
	cfgFile    string
	doLog      bool
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

type Host struct {
	Hostname string `json:"Hostname"`
	VDirs    []VDir `json:"VDirs"`
}

type VDir struct {
	Root   string `json:"Root"`
	Folder string `json:"Folder"`
}

type Config struct {
	Addr  string `json:"Addr"`
	Hosts []Host `json:"Hosts"`
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
	if len(c.Hosts) == 0 {
		log.Fatalf("No Hosts specified in configuration file %s", cfgFile)
	}
	for _, h := range c.Hosts {
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
				log.Fatalf("Invalid Folder specified in configuration file %s for host \"%s\": \"%s\"", cfgFile, h.Hostname, v.Folder)
			}
			v.Root = strings.TrimSpace(v.Root)
			if v.Root == "" {
				log.Fatalf("Invalid Root specified in configuration file %s for host \"%s\": \"%s\"", cfgFile, h.Hostname, v.Root)
			}
		}
	}
	c.Addr = strings.TrimSpace(c.Addr)
	if c.Addr == "" {
		c.Addr = ":8080"
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
	flag.BoolVar(&svcInstall, "install", false, "Install the AutoPoster as a service")
	flag.BoolVar(&svcRemove, "remove", false, "Remove the AutoPoster service")
	flag.BoolVar(&svcRun, "run", false, "Run the AutoPoster standalone (not as a service)")
	flag.BoolVar(&svcStart, "start", false, "Start the AutoPoster service")
	flag.BoolVar(&svcStop, "stop", false, "Stop the AutoPoster service")
	flag.BoolVar(&doLog, "log", false, "Log requests")

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
	hostMap := make(map[string]*http.ServeMux, 0)
	for _, host := range conf.Hosts {
		mux := http.NewServeMux()
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
	if err := http.ListenAndServe(conf.Addr, SelectHost(hostMap)); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
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

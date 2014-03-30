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

type VDir struct {
	Root   string `json:"Root"`
	Folder string `json:"Folder"`
}

type Config struct {
	Addr  string `json:"Addr"`
	VDirs []VDir `json:"VDirs"`
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
	if len(c.VDirs) == 0 {
		log.Fatalf("No VDirs specified in configuration file %s", cfgFile)
		for _, f := range c.VDirs {
			if strings.TrimSpace(f.Folder) != f.Folder || strings.TrimSpace(f.Folder) == "" {
				log.Fatalf("Invalid Folder specified in configuration file %s: \"%s\"", cfgFile, f.Folder)
			}
			if strings.TrimSpace(f.Root) != f.Root || strings.TrimSpace(f.Root) == "" {
				log.Fatalf("Invalid Root specified in configuration file %s: \"%s\"", cfgFile, f.Root)
			}
		}
	}
	if strings.TrimSpace(c.Addr) == "" {
		c.Addr = ":8080"
	}
	return
}

type RequestLogger struct {
	h http.Handler
	log bool
}

func (rl RequestLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rl.log {
		log.Print(r.Host, r.URL)
	}
	rl.h.ServeHTTP(w, r)
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
	for _, f := range conf.VDirs {
		if f.Root == "/" {
			http.Handle(f.Root, RequestLogger{http.FileServer(http.Dir(f.Folder)), doLog})
		} else {
			http.Handle(f.Root, RequestLogger{http.StripPrefix(f.Root, http.FileServer(http.Dir(f.Folder))), doLog})
		}
	}
	if err := http.ListenAndServe(conf.Addr, nil); err != nil {
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

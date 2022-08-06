package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type Page struct {
	Title      string
	Running    []FinalModel
	NotRunning []FinalModel
	IsRound    string
	Hostname   string
	Wan        bool
}

type FinalModel struct {
	Name      string
	Icon      string
	WebuiLan  string
	WebuiWan  string
	Running   bool
	Shell     string
	SubDomain string
}

//type DockerStart struct {
//	Message  string `json:"message"`
//	Error    string `json:"err"`
//	UnraidIP string `json:"unraid_ip"`
//}

//go:embed html
var content embed.FS

//go:embed static
var staticAssets embed.FS

var pathFile = "config/subdomains.yml"

var WAN = false

func main() {

	if os.Getenv("DOCKER_PATH") == "" {
		pathFile = "/" + pathFile
	}
	if strings.ToLower(os.Getenv("WAN")) == "true" {
		WAN = true
	} else {
		WAN = false
	}

	//WAN = true

	file, err := os.OpenFile(pathFile, os.O_CREATE|os.O_APPEND, 0644)
	defer file.Close()

	if err != nil {
		log.Println(err)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir("/data/images"))
	mux.Handle("/images/", http.StripPrefix("/images", fileServer))

	mux.Handle("/static/", http.StripPrefix("/", http.FileServer(http.FS(staticAssets))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		running, notRunning := getDocker()
		var page = Page{
			Title: func() string {
				if os.Getenv("TITLE") == "" {
					return "Docker WebUI"
				} else {
					return os.Getenv("TITLE")
				}
			}(),
			Running:    running,
			NotRunning: notRunning,
			IsRound:    os.Getenv("CIRCLE"),
			Wan:        WAN,
		}

		t, err := template.ParseFS(content, "html/index.gohtml")
		if err != nil {
			log.Println(err)
			return
		}

		err = t.Execute(w, page)
		if err != nil {
			log.Println(err)
			return
		}
	})

	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "", http.StatusInternalServerError)
				log.Println(err)
				return
			}
			type respBody struct {
				Title     string `json:"title"`
				SubDomain string `json:"sub_domain"`
			}
			var resp = &respBody{}
			err = json.Unmarshal(body, &resp)
			if err != nil {
				http.Error(w, "", http.StatusInternalServerError)
				log.Println(err)
				return
			}

			var docker = &Docker{}
			var container = &Container{
				Name: resp.Title,
				Options: &Options{
					SubDomain: resp.SubDomain,
				},
			}

			docker.update(container)
			w.WriteHeader(http.StatusCreated)
			return
		}
	})

	//mux.HandleFunc("/docker-start", func(w http.ResponseWriter, r *http.Request) {
	//	if r.Method == http.MethodPost {
	//		title := r.FormValue("title")
	//		_, err := exec.Command("docker", "start", title).Output()
	//		dockerStart := DockerStart{
	//			UnraidIP: os.Getenv("UNRAID_IP"),
	//		}
	//		if err != nil {
	//			log.Println(fmt.Sprintf("Problem : The '%s' container will not be started. %v", title, err))
	//			dockerStart.Error = fmt.Sprintf("Problem : The '%s' container will not be started", title)
	//			json.NewEncoder(w).Encode(dockerStart)
	//			return
	//		}
	//
	//		dockerStart.Message = fmt.Sprintf("The '%s' container will be started", title)
	//		json.NewEncoder(w).Encode(dockerStart)
	//		return
	//	}
	//})

	port := func() string {
		if os.Getenv("PORT") == "" {
			return "localhost:8080"
		} else {
			return ":" + os.Getenv("PORT")
		}
	}
	log.Println("Started web to port " + port())
	log.Fatalln(http.ListenAndServe(port(), mux))
}

func getDocker() (running, notRunning []FinalModel) {

	//config, err := ioutil.ReadFile("./docker.json")
	var pathDocker string
	if os.Getenv("DOCKER_PATH") == "" {
		pathDocker = "/data/docker.json"
	} else {
		pathDocker = os.Getenv("DOCKER_PATH")
	}
	data, err := ioutil.ReadFile(pathDocker)
	if err != nil {
		fmt.Print(err)
	}

	var payload interface{}
	err = json.Unmarshal(data, &payload)
	if err != nil {
		log.Println(err)
	}
	m := payload.(map[string]interface{})

	checkName := os.Getenv("HOST_CONTAINERNAME")
	if checkName == "" {
		checkName = "Docker-WebUI"
	}

	for k, v := range m {
		model := v.(map[string]interface{})
		var run FinalModel
		for s, vv := range model {
			run.Name = k
			switch s {
			case "icon":
				run.Icon = path.Clean("/images/" + path.Base(checkIfNotNullAndReturnString(vv)))
			case "url":
				uu, err := url.Parse(checkIfNotNullAndReturnString(vv))

				if err != nil {
					log.Println(err)
				}
				if uu.Host != "" {
					u := strings.Split(uu.Host, ":")
					run.WebuiLan = uu.Scheme + "://" + uu.Host
					if os.Getenv("HOST") != "" {
						uu.Host = os.Getenv("HOST")
						if len(u) == 2 {
							uu.Host = uu.Host + ":" + u[1]
						}
						run.WebuiWan = uu.String()
						//} else if os.Getenv("HOST") != "" && os.Getenv("UNRAID_IP") != "" {
						//	if u[0] == os.Getenv("UNRAID_IP") && len(u) == 2 {
						//		uu.Host = os.Getenv("HOST")
						//		uu.Host = uu.Host + ":" + u[1]
						//	}
						//	run.WebuiLan = uu.String()
					} else {
						run.WebuiLan = checkIfNotNullAndReturnString(vv)
					}
				}
			case "running":
				b, err := regexp.Compile(`(?i)^true$|^false$`)
				if err != nil {
					log.Println(err)
					run.Running = false
				} else {
					if b.MatchString(fmt.Sprintf("%v", vv)) {
						run.Running = vv.(bool)
					} else {
						run.Running = false
					}
					continue
				}
			case "shell":
				if vv != nil {
					run.Shell = vv.(string)
				} else {
					run.Shell = "sh"
				}

			}
		}
		// Update for version 6.10-rc2 or newer => os.Getenv("HOST_CONTAINERNAME")

		if (run.WebuiLan != "" || run.WebuiWan != "") && run.Name != checkName {
			if run.Running {
				running = append(running, run)
			} else {
				notRunning = append(notRunning, run)
			}
		}
	}
	sort.Slice(running, func(i, j int) bool {
		return strings.ToLower(running[i].Name) < strings.ToLower(running[j].Name)
	})
	sort.Slice(notRunning, func(i, j int) bool {
		return strings.ToLower(notRunning[i].Name) < strings.ToLower(notRunning[j].Name)
	})

	var docker = &Docker{}
	docker.read()

	for _, c := range docker.Containers {
		for i, r := range running {
			if c.Name == r.Name {
				running[i].SubDomain = c.Options.SubDomain
			}
		}
		for i, n := range notRunning {
			if c.Name == n.Name {
				notRunning[i].SubDomain = c.Options.SubDomain
			}
		}
	}

	log.Printf("App Runing : %+v\n", running)
	log.Printf("App Not running: %+v\n", notRunning)
	return running, notRunning
}

func checkIfNotNullAndReturnString(vv interface{}) string {
	if vv != nil {
		s := vv.(string)
		return strings.Replace(s, "&amp;", "&", -1)
	}
	return ""
}

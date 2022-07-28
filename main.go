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
	"os/exec"
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
}

type FinalModel struct {
	Name    string
	Icon    string
	Webui   string
	Running bool
	Shell   string
}

type DockerStart struct {
	Message  string `json:"message"`
	Error    string `json:"err"`
	UnraidIP string `json:"unraid_ip"`
}

//go:embed html
var content embed.FS

//go:embed static/favicon.ico
var staticAssets embed.FS

func main() {

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

	mux.HandleFunc("/docker-start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			title := r.FormValue("title")
			_, err := exec.Command("docker", "start", title).Output()
			dockerStart := DockerStart{
				UnraidIP: os.Getenv("UNRAID_IP"),
			}
			if err != nil {
				log.Println(fmt.Sprintf("Problem : The '%s' container will not be started. %v", title, err))
				dockerStart.Error = fmt.Sprintf("Problem : The '%s' container will not be started", title)
				json.NewEncoder(w).Encode(dockerStart)
				return
			}

			dockerStart.Message = fmt.Sprintf("The '%s' container will be started", title)
			json.NewEncoder(w).Encode(dockerStart)
			return
		}
	})

	port := func() string {
		if os.Getenv("PORT") == "" {
			return ":8080"
		} else {
			return ":" + os.Getenv("PORT")
		}
	}
	log.Println("Started web to port " + port())
	log.Fatalln(http.ListenAndServe(port(), mux))
}

func getDocker() (running, notRunning []FinalModel) {

	//data, err := ioutil.ReadFile("./docker.json")
	data, err := ioutil.ReadFile("/data/docker.json")
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
					if os.Getenv("HOST") != "" && os.Getenv("UNRAID_IP") == "" {
						uu.Host = os.Getenv("HOST")
						if len(u) == 2 {
							uu.Host = uu.Host + ":" + u[1]
						}
						run.Webui = uu.String()
					} else if os.Getenv("HOST") != "" && os.Getenv("UNRAID_IP") != "" {
						if u[0] == os.Getenv("UNRAID_IP") && len(u) == 2 {
							uu.Host = os.Getenv("HOST")
							uu.Host = uu.Host + ":" + u[1]
						}
						run.Webui = uu.String()
					} else {
						run.Webui = checkIfNotNullAndReturnString(vv)
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

		if run.Webui != "" && run.Name != checkName {
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
	log.Printf("App Runing : %v\n", running)
	log.Printf("App Not running: %v\n", notRunning)
	return running, notRunning
}

func checkIfNotNullAndReturnString(vv interface{}) string {
	if vv != nil {
		s := vv.(string)
		return strings.Replace(s, "&amp;", "&", -1)
	}
	return ""
}

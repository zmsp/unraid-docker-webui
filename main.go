package main

import (
	"embed"
	_ "embed"
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

//go:embed html
var content embed.FS

func main() {
	fileServer := http.FileServer(http.Dir("/data/images"))
	http.Handle("/images/", http.StripPrefix("/images", fileServer))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		running, notRunning := getDocker()
		var page = Page{
			Running:    running,
			NotRunning: notRunning,
			IsRound:    os.Getenv("CIRCLE"),
		}

		t, err := template.ParseFS(content, "html/index.html")
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

	log.Println("Started web to port 1111")
	log.Fatalln(http.ListenAndServe(":1111", nil))
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

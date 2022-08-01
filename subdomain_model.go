package main

import (
	"github.com/goccy/go-yaml"
	"io/ioutil"
	"log"
)

type Docker struct {
	Containers []*Container `yaml:"containers, omitempty"`
}

type Container struct {
	Name    string `yaml:"name, omitempty"`
	Options *Options
}

type Options struct {
	SubDomain string `yaml:"sub_domain, omitempty"`
}

func (d *Docker) write() {

	y, err := yaml.Marshal(d)
	if err != nil {
		log.Println(err)
	}

	err = ioutil.WriteFile(pathFile, y, 0644)
	if err != nil {
		log.Println(err)
	}
}

func (d *Docker) read() *Docker {

	b, err := ioutil.ReadFile(pathFile)
	if err != nil {
		log.Println(err)
	}

	err = yaml.Unmarshal(b, d)
	if err != nil {
		log.Println(err)
	}
	return d
}

func (d *Docker) update(container *Container) {
	d.read()
	isExist := false
	for i, c := range d.Containers {
		if c.Name == container.Name {
			d.Containers[i] = container
			isExist = true
			break
		}
	}
	if !isExist {
		d.Containers = append(d.Containers, container)
	} else if isExist && container.Options.SubDomain == "" {
		d.delete(container)
	}
	d.write()
}

func (d *Docker) delete(container *Container) {
	d.read()
	var l []*Container
	for _, c := range d.Containers {
		if c.Name != container.Name {
			l = append(l, c)
		}
	}
	d.Containers = l
	d.write()
}

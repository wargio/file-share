// SPDX-FileCopyrightText: 2018-2024 deroad <wargio@libero.it>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

var (
	nameUri = map[string]string{}
	uriFile = map[string]string{}
)

func RandString(n int) string {
	const encoding = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const modulo = byte(len(encoding))
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	for i, b := range bytes {
		bytes[i] = encoding[b%modulo]
	}
	return string(bytes)
}

func loadAsset(file string) ([]byte, error) {
	if strings.HasSuffix(file, ".tmpl") {
		return nil, nil
	}
	asset, err := Assets.Open(file)
	if err != nil {
		return nil, nil
	}
	content, err := ioutil.ReadAll(asset)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func loadEmbedded() (*template.Template, error) {
	t := template.New("")
	for name, file := range Assets.Files {
		if file.IsDir() || !strings.HasSuffix(name, ".tmpl") {
			continue
		}
		h, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
		t, err = t.New(path.Base(name)).Parse(string(h))
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func detectContentType(file string, content []byte) string {
	if strings.HasSuffix(file, ".css") {
		return "text/css"
	} else if strings.HasSuffix(file, ".js") {
		return "text/javascript"
	}
	return http.DetectContentType(content)
}

func main() {
	var bind string
	var debug bool

	flag.StringVar(&bind, "bind", ":8080", "[address]:[port] address to bind to.")
	flag.BoolVar(&debug, "debug", false, "enable http debug logs.")
	flag.Parse()
	files := flag.Args()

	if len(files) < 1 {
		panic("no files where supplied as argument")
	}

	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			panic(err)
		}
		name := path.Base(file)
		uri := "/share/" + RandString(16) + "/" + name
		nameUri[name] = uri
		uriFile[uri] = file
	}

	gin.DisableConsoleColor()
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	templates, err := loadEmbedded()
	if err != nil {
		panic(err)
	}
	router.SetHTMLTemplate(templates)
	router.GET("/static/:file", func(c *gin.Context) {
		file := c.Param("file")
		content, err := loadAsset(file)
		if content == nil && err == nil {
			c.Status(404)
			return
		} else if err != nil {
			c.Status(500)
			fmt.Println("[Assets]", err)
			return
		}
		contentType := detectContentType(file, content)
		c.Data(200, contentType, content)
	})
	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.tmpl", gin.H{
			"files": nameUri,
		})
	})
	for uri, file := range uriFile {
		router.StaticFile(uri, file)
	}
	router.Run(bind)
}

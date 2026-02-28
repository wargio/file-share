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
	"path/filepath"
	"strings"
	"time"
)

var (
	nameUri   = map[string]string{}
	uriFile   = map[string]string{}
	uploadDir string
	bind      string
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

func addFile(userPath string) {
	name := path.Base(userPath)
	uri := "/share/" + RandString(16) + "/" + name
	nameUri[name] = uri
	uriFile[uri] = userPath
	// preview path on the terminal
	prefix := "http://"
	if bind[0] == ':' {
		prefix += "127.0.0.1"
	}
	fmt.Println(prefix + bind + uri)
}

func walkDir(userPath string) {
	filepath.Walk(userPath, func(wpath string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		addFile(wpath)
		return nil
	})
}

func main() {
	var debug, upload bool

	flag.StringVar(&bind, "bind", ":8080", "[address]:[port] address to bind to.")
	flag.BoolVar(&debug, "debug", false, "enable http debug logs.")
	flag.BoolVar(&upload, "upload", false, "act as upload server")
	flag.Parse()
	args := flag.Args()

	if upload {
		if len(args) != 1 {
			panic("please provide a destination folder.")
		}
		var err error
		uploadDir, err = filepath.Abs(args[0])
		s, err := os.Stat(uploadDir)
		if err != nil {
			panic(err)
		} else if !s.IsDir() {
			panic("please provide a valid destination folder: " + uploadDir + " is not a directory")
		}
	} else {
		if len(args) < 1 {
			panic("no files where supplied as argument")
		}
		for _, userPath := range args {
			stat, err := os.Stat(userPath)
			if err != nil {
				panic(err)
			}
			if stat.IsDir() {
				walkDir(userPath)
			} else {
				addFile(userPath)
			}
		}
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
	if upload {
		router.POST("/upload", func(c *gin.Context) {
			file, _ := c.FormFile("file")
			name := strings.TrimSpace(path.Base(file.Filename))
			if name == "" {
				name = "bin_" + time.Now().Format("2006.01.02_15.04.05")
			}
			dst := filepath.Join(uploadDir, name)

			fmt.Println("uploading to", dst)

			c.SaveUploadedFile(file, dst)
			c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", name))
		})
	}
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
	if upload {
		router.GET("/", func(c *gin.Context) {
			c.HTML(200, "upload.tmpl", gin.H{
				"files": nameUri,
			})
		})
	} else {
		router.GET("/", func(c *gin.Context) {
			c.HTML(200, "index.tmpl", gin.H{
				"files": nameUri,
			})
		})
		for uri, file := range uriFile {
			router.StaticFile(uri, file)
		}
	}
	prefix := ""
	if bind[0] == ':' {
		prefix += "127.0.0.1"
	}
	fmt.Println("server running: http://" + prefix + bind)
	router.Run(bind)
}

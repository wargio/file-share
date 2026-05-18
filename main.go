// SPDX-FileCopyrightText: 2018-2026 deroad <wargio@libero.it>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"embed"
	"errors"
	"io/fs"
)

type VarMap map[string]string

var (
	nameUri   = map[string]string{}
	nameDate  = map[string]string{}
	uriFile   = map[string]string{}
	uploadDir string
	bind      string
	quiet     bool
	basicauth = VarMap{}
	//go:embed embedded/*
	embedded embed.FS
)

func(ma *VarMap) String() string {
	return fmt.Sprintf("%v", *ma)
}

func (ma *VarMap) Set(value string) error {
	tok := strings.SplitN(value, ":", 2)
	if len(tok) != 2 {
		return errors.New("missing : for the password")
	}
    (*ma)[tok[0]] = tok[1]
    return nil
}

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

	subpath := path.Join("embedded", file)
	return embedded.ReadFile(subpath)
}

func loadEmbedded() (*template.Template, error) {
	t := template.New("")
	err := fs.WalkDir(embedded, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() || !strings.HasSuffix(name, ".tmpl") {
			return nil
		}
		content, err := embedded.ReadFile(name)
		if err != nil {
			return err
		}
		t, err = t.New(path.Base(name)).Parse(string(content))
		if err != nil {
			return err
		}
		return nil
	})
	return t, err
}

func detectContentType(file string, content []byte) string {
	if strings.HasSuffix(file, ".css") {
		return "text/css"
	} else if strings.HasSuffix(file, ".js") {
		return "text/javascript"
	}
	return http.DetectContentType(content)
}

func addFile(userPath string, modTime time.Time) {
	name := path.Base(userPath)
	uri := "/share/" + RandString(16) + "/" + name
	nameUri[name] = uri
	nameDate[name] = modTime.Format(time.RFC1123)
	uriFile[uri] = userPath
	if !quiet {
		// preview path on the terminal
		prefix := "http://"
		if bind[0] == ':' {
			prefix += "127.0.0.1"
		}
		fmt.Println(prefix + bind + uri)
	}
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
		addFile(wpath, info.ModTime())
		return nil
	})
}

func main() {
	var debug, upload bool

	flag.StringVar(&bind, "bind", ":8080", "[address]:[port] address to bind to.")
	flag.BoolVar(&debug, "debug", false, "enable http debug logs.")
	flag.BoolVar(&quiet, "quiet", false, "makes the output more quiet.")
	flag.BoolVar(&upload, "upload", false, "act as upload server")
	flag.Var(&basicauth, "auth", "adds basic auth user, example `-auth user:password`.")
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
				addFile(userPath, stat.ModTime())
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

	webUi := router.Group("/ui")

    router.GET("/", func(c *gin.Context) {
        c.Redirect(http.StatusMovedPermanently, "/ui/")
    })

	if len(basicauth) > 0 {
		ga := gin.Accounts{}
		for key, val := range basicauth {
			ga[key] = val
		}
		webUi.Use(gin.BasicAuth(ga))
	}

	templates, err := loadEmbedded()
	if err != nil {
		panic(err)
	}
	router.SetHTMLTemplate(templates)
	if upload {
		webUi.POST("/upload", func(c *gin.Context) {
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

	webUi.GET("static/:file", func(c *gin.Context) {
		file := c.Param("file")
		if len(file) < 1 {
			c.Status(400)
			return
		}
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
		webUi.GET("/", func(c *gin.Context) {
			c.HTML(200, "upload.tmpl", gin.H{})
		})
	} else {
		webUi.GET("/", func(c *gin.Context) {
			c.HTML(200, "index.tmpl", gin.H{
				"name_uri": nameUri,
				"name_date": nameDate,
			})
		})
		router.GET("/share/*file", func(c *gin.Context) {
			file := c.Param("file")
			if len(file) < 1 {
				c.Status(400)
				return
			}

			fileloc, ok := uriFile["/share"+file]
			if !ok {
				c.Status(404)
				return
			}

			if c.DefaultQuery("bin", "false") == "true" {
				c.Header("Content-Type", "application/octet-stream")
			}
			c.File(fileloc)
		})
	}
	if !quiet {
		prefix := ""
		if bind[0] == ':' {
			prefix += "127.0.0.1"
		}
		fmt.Println("server running: http://" + prefix + bind)
	}
	router.Run(bind)
}

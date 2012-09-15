/*

	This file is part of Ninja Go Local Cloud (https://pacien.net/projects/Ninja Go Local Cloud).

	Ninja Go Local Cloud is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	Ninja Go Local Cloud is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with Ninja Go Local Cloud. If not, see <http://www.gnu.org/licenses/>.

*/

package main

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const APP_NAME = "Ninja Go Local Cloud"
const APP_VERSION = "0.1 Draft"

var versionFlag bool
var interfaceFlag string
var portFlag string
var rootFlag string

const filePath = "/file/"
const dirPath = "/directory/"
const webPath = "/web?url="
const statusPath = "/cloudstatus"

const filePathLen = len(filePath)
const dirPathLen = len(dirPath)
const webPathLen = len(webPath)

//const statusPathLen = len(statusPath)

func sliceContains(s []string, c string) bool {
	for _, e := range s {
		if c == e {
			return true
		}
	}
	return false
}

//////// FILESYSTEM

func properties(path string) (infos os.FileInfo, err error) {
	infos, err = os.Stat(path)
	return
}

func modifiedSince(path string, since string) bool {
	s, err := strconv.ParseInt(since, 10, 64)
	infos, err := properties(path)
	if err != nil {
		return false
	}
	l := infos.ModTime().UnixNano()
	s = s * 1000000
	if s > l {
		return true
	}
	return false
}

func exist(path string) bool {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return true
	}
	return false
}

func isInRoot(path string) bool {
	return filepath.HasPrefix(path, rootFlag)
}

//// Files

func writeFile(path string, content []byte, overwrite bool) (err error) {
	if !overwrite {
		if exist(path) {
			err = os.ErrExist
			return
		}
	} else {
		if !exist(path) {
			err = os.ErrNotExist
			return
		}
	}
	err = ioutil.WriteFile(path, content, 0600)
	return
}

/*func readFile(path string) (content []byte, err error) {
	content, err = ioutil.ReadFile(path)
	return
}*/

func removeFile(path string) (err error) {
	err = os.Remove(path)
	return
}

func moveFile(source string, dest string) (err error) {
	err = os.Rename(source, dest)
	return
}

func copyFile(source string, dest string) (err error) {
	// from https://gist.github.com/2876519
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = io.Copy(df, sf)
	if err == nil {
		si, err := os.Stat(source)
		if err != nil {
			err = os.Chmod(dest, si.Mode())
		}

	}
	return
}

//// Dirs

func createDir(path string) (err error) {
	err = os.MkdirAll(path, 0600)
	return
}

func removeDir(path string) (err error) {
	err = os.RemoveAll(path)
	return
}

/*func listDir(path string) (list []os.FileInfo, err error) {
	list, err = ioutil.ReadDir(path)
	return
}*/

func moveDir(source string, dest string) (err error) {
	err = os.Rename(source, dest)
	return
}

func copyDir(source string, dest string) (err error) {
	// from https://gist.github.com/2876519
	fi, err := os.Stat(source)
	if err != nil {
		return
	}
	if !fi.IsDir() {
		return os.ErrInvalid
	}
	_, err = os.Open(dest)
	if !os.IsNotExist(err) {
		return os.ErrExist
	}
	err = os.MkdirAll(dest, fi.Mode())
	if err != nil {
		return
	}
	entries, err := ioutil.ReadDir(source)
	for _, entry := range entries {
		sfp := source + "/" + entry.Name()
		dfp := dest + "/" + entry.Name()
		if entry.IsDir() {
			err = copyDir(sfp, dfp)
			if err != nil {
				return
			}
		} else {
			err = copyFile(sfp, dfp)
			if err != nil {
				return
			}
		}
	}
	return
}

type element struct {
	Type         string    `json:"type"`
	Name         string    `json:"name"`
	Uri          string    `json:"uri"`
	CreationDate string    `json:"creationdate"`
	ModifiedDate string    `json:"modifieddate"`
	Size         string    `json:"size"`
	Writable     string    `json:"writable"`
	Children     []element `json:"children"`
}

func listDir(path string, recursive bool, filter []string, returnType string) (list []element, err error) {
	returnAll := returnType == "all" || returnType == ""
	returnFiles := returnType == "files" || returnAll
	returnDirs := returnType == "directories" || returnAll
	currentDir, err := ioutil.ReadDir(path)
	for _, d := range currentDir {
		if d.IsDir() && returnDirs {
			var e element
			modTime := strconv.FormatInt(d.ModTime().UnixNano(), 10)
			modTime = modTime[:len(modTime)-6]
			uri := filepath.Clean(path + "/" + d.Name())
			list = append(list, element{})
			e.Type = "directory"
			e.Name = d.Name()
			e.Uri = uri
			e.CreationDate = modTime // TODO
			e.ModifiedDate = modTime
			e.Size = strconv.FormatInt(d.Size(), 10)
			e.Writable = "true" // TODO
			if recursive {
				e.Children, err = listDir(uri, recursive, filter, returnType)
				if err != nil {
					return
				}
			} else {
				e.Children = nil
			}
			list = append(list, e)
		} else if !d.IsDir() && returnFiles {
			if sliceContains(filter, filepath.Ext(d.Name())) {
				var e element
				modTime := strconv.FormatInt(d.ModTime().UnixNano(), 10)
				modTime = modTime[:len(modTime)-6]
				e.Type = "file"
				e.Name = d.Name()
				e.Uri = filepath.Clean(path + "/" + d.Name())
				e.CreationDate = modTime // TODO
				e.ModifiedDate = modTime
				e.Size = strconv.FormatInt(d.Size(), 10)
				e.Writable = "true" // TODO
				e.Children = nil
				list = append(list, e)
			}
		}
	}
	return
}

//////// REQUEST HANDLERS

func osPath(p string) string {
	filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = p[:1] + ":" + p[1:]
	} else {
		p = "/" + p
	}
	return p
}

//// File APIs

func fileHandler(w http.ResponseWriter, r *http.Request) {
	p := osPath(r.URL.Path[filePathLen:])
	if !isInRoot(p) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	switch r.Method {
	case "POST":
		// Create a new file
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = writeFile(p, *&content, false)
		if err == os.ErrExist {
			w.WriteHeader(http.StatusBadRequest)
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	case "PUT":
		source := r.Header.Get("sourceURI")
		if source == "" {
			// Update an existing file (save over existing file)
			content, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			err = writeFile(p, *&content, true)
			if err == os.ErrNotExist {
				w.WriteHeader(http.StatusNotFound)
			} else if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		} else {
			// Copy, Move of an existing file 
			if r.Header.Get("overwrite-destination") == "true" {
				err := removeFile(p)
				if err == os.ErrNotExist {
					w.WriteHeader(http.StatusNotFound)
					return
				} else if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			if r.Header.Get("delete-source") == "true" {
				err := moveFile(source, p)
				if err == os.ErrNotExist {
					w.WriteHeader(http.StatusNotFound)
					return
				} else if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			} else {
				err := copyFile(source, p)
				if err == os.ErrNotExist {
					w.WriteHeader(http.StatusNotFound)
					return
				} else if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	case "DELETE":
		// Delete an existing file
		err := removeFile(p)
		if err == os.ErrNotExist {
			w.WriteHeader(http.StatusNotFound)
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case "GET":
		// Read an existing file
		modSince := r.Header.Get("If-modified-since")
		getInfo := r.Header.Get("get-file-info")
		if modSince != "" && modSince != "false" && modSince != "none" {
			if modifiedSince(p, modSince) {
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		} else if r.Header.Get("check-existence-only") == "true" {
			if exist(p) {
				w.WriteHeader(http.StatusNoContent)
				return
			} else {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		} else if getInfo != "" && getInfo != "false" {
			infos, err := properties(p)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			modDate := strconv.FormatInt(infos.ModTime().UnixNano(), 10)
			modDate = modDate[:len(modDate)-6]
			size := strconv.FormatInt(infos.Size(), 10)
			fileInfo := map[string]string{
				"creationDate": modDate, // TODO
				"modifiedDate": modDate,
				"size":         size,
				"readOnly":     "false", // TODO
			}
			j, err := json.Marshal(fileInfo)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(j)
			return
		} else {
			http.ServeFile(w, r, p)
		}
	}
}

//// Directory APIs

func dirHandler(w http.ResponseWriter, r *http.Request) {
	p := osPath(r.URL.Path[dirPathLen:])
	if !isInRoot(p) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	switch r.Method {
	case "POST":
		// Create a new directory
		err := createDir(p)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	case "DELETE":
		// Delete an existing directory
		err := removeDir(p)
		if err == os.ErrNotExist {
			w.WriteHeader(http.StatusNotFound)
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case "GET":
		// List the contents of an existing directory
		modSince := r.Header.Get("If-modified-since")
		if p == "" {
			w.Write([]byte(rootFlag))
			return
		} else if modSince != "" && modSince != "false" && modSince != "none" {
			if !exist(p) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if modifiedSince(p, modSince) {
				w.WriteHeader(http.StatusOK)
				return
			} else {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		} else if r.Header.Get("check-existence-only") == "true" {
			if exist(p) {
				w.WriteHeader(http.StatusNoContent)
				return
			} else {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		} else {
			recursive := r.Header.Get("recursive") == "true"
			filter := strings.Split(r.Header.Get("file-filters"), ";")
			returnType := r.Header.Get("return-type")
			if returnType == "" {
				returnType = "all"
			}
			fileInfo, err := listDir(p, recursive, filter, returnType)
			if err == os.ErrNotExist {
				w.WriteHeader(http.StatusNotFound)
				return
			} else if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var e element
			e.Type = "directory"
			e.Name = "root"
			e.Uri = p + "/"
			e.Children = fileInfo
			j, err := json.Marshal(e)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(j)
			return
		}
	case "PUT":
		// Copy, Move of an existing directory
		source := r.Header.Get("sourceURI")
		if exist(p) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		operation := r.Header.Get("operation")
		if operation == "move" {
			err := moveDir(source, p)
			if err == os.ErrNotExist {
				w.WriteHeader(http.StatusNotFound)
				return
			} else if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else if operation == "copy" {
			err := copyDir(source, p)
			if err == os.ErrNotExist {
				w.WriteHeader(http.StatusNotFound)
				return
			} else if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
}

//// Web API

// Get text or binary data from a URL
func getDataHandler(w http.ResponseWriter, r *http.Request) {
}

//// Cloud Status API

// Get the cloud status JSON
func getStatusHandler(w http.ResponseWriter, r *http.Request) {
	cloudStatus := map[string]string{
		"name":        APP_NAME,
		"version":     APP_VERSION,
		"server-root": rootFlag,
		"status":      "running",
	}
	j, err := json.Marshal(cloudStatus)
	if err != nil {
		log.Println(err)
	}
	w.Write(j)
}

//////// INIT and MAIN

func init() {
	flag.BoolVar(&versionFlag, "v", false, "Print the version number.")
	flag.StringVar(&interfaceFlag, "i", "localhost", "Listening interface.")
	flag.StringVar(&portFlag, "p", "58080", "Listening port.")
	flag.StringVar(&rootFlag, "r", ".", "Root directory.")
}

func main() {
	flag.Parse()

	if versionFlag {
		log.Println("Version:", APP_VERSION)
		return
	}

	log.Println("Starting " + APP_NAME + " " + APP_VERSION + " on " + interfaceFlag + ":" + portFlag + " in " + rootFlag)

	http.HandleFunc(filePath, fileHandler)
	http.HandleFunc(dirPath, dirHandler)
	http.HandleFunc(webPath, getDataHandler)
	http.HandleFunc(statusPath, getStatusHandler)

	err := http.ListenAndServe(interfaceFlag+":"+portFlag, nil)
	if err != nil {
		log.Println(err)
		return
	}
}

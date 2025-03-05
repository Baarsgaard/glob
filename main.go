package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

//go:embed index.html
var index string
var log *zap.Logger

func writeToDisk(filename string, content []byte) error {
	// Open and truncate or create file
	// TODO Make storage path configurable
	file, err := os.OpenFile("./files/"+filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write file
	_, err = file.Write(content)
	if err != nil {
		return err
	}

	log.Debug("Content written to: " + filename)

	return nil
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	log := log.With(
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	if r.Body == nil {
		log.Error("Bad request, empty body")
		http.Error(w, http.StatusText(http.StatusBadRequest)+"Empty body", http.StatusBadRequest)
		return
	}

	filename := r.PathValue("file")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest)+err.Error(), http.StatusBadRequest)
		return
	}

	err = writeToDisk(filename, body)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError)+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TODO Refactor
func uploadFormFile(w http.ResponseWriter, r *http.Request) {
	log := log.With(
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	// Upload limit: 10Mi
	// TODO Make configurable
	err := r.ParseMultipartForm(0xA00000)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, http.ErrContentLength.Error()+err.Error(), http.StatusBadRequest)
		return
	}

	// Get reqFile form fields
	reqFile, handler, err := r.FormFile("filename")
	if err != nil {
		log.Error(err.Error())
		http.Error(w, http.ErrMissingFile.Error()+err.Error(), http.StatusBadRequest)
		return
	}
	defer reqFile.Close()

	// read file contents
	fileBytes, err := io.ReadAll(reqFile)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Debug("File received",
		zap.String("filename", handler.Filename),
		zap.Int64("size", handler.Size),
		zap.String("MIME", http.DetectContentType(fileBytes)),
	)

	err = writeToDisk(handler.Filename, fileBytes)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError)+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func listFiles(w http.ResponseWriter, r *http.Request) {
	log := log.With(
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	accept := strings.Split(r.Header.Get("Accept"), ",")[0]
	log.Debug("Accept Header:" + accept)
	isJson := false
	switch accept {
	case "text/html": // Browser
		http.ServeFile(w, r, "./files")
	case "application/json":
		fallthrough
	case "text/json": // Asked for
		isJson = true
		fallthrough
	default: // */* Curl and anything else
		entries, err := os.ReadDir("./files")
		if err != nil {
			log.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError)+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isJson {
			for _, entry := range entries {
				fmt.Fprintln(w, entry.Name())
			}
			return
		}

		var files []string
		for _, entry := range entries {
			files = append(files, entry.Name())
		}

		json, err := json.Marshal(files)
		if err != nil {
			log.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError)+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, string(json))
	}
}

func getFile(w http.ResponseWriter, r *http.Request) {
	log := log.With(
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	file := r.PathValue("file")
	if file == "index.html" {
		fmt.Fprint(w, index)
		return
	}

	http.ServeFile(w, r, filepath.Join("files", file))
	log.Debug("Served " + file)
}

func deleteFile(w http.ResponseWriter, r *http.Request) {
	log := log.With(
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)
	file := r.PathValue("file")
	err := os.RemoveAll(file)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, "Failed to delete file:"+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Debug("Deleted " + file)
}

func main() {
	dbg := os.Getenv("DEBUG")

	var err error
	if dbg != "" {
		log, err = zap.NewDevelopment()
	} else {
		log, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Println(err.Error())
	}
	defer log.Sync() // nolint:errcheck

	log = log.Named("Glob")

	// TODO Make storage path configurable
	err = os.MkdirAll("./files", os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
	}

	// GET/PUT/POST/DELETE
	// POST behaves like a PUT, ergonomic over correctness
	http.HandleFunc("GET /", listFiles)
	http.HandleFunc("GET /{file}", getFile)
	http.HandleFunc("POST /", uploadFormFile)
	http.HandleFunc("POST /{file}", uploadFile)
	http.HandleFunc("PUT /", uploadFormFile)
	http.HandleFunc("PUT /{file}", uploadFile)
	http.HandleFunc("DELETE /{file}", deleteFile)

	fmt.Println("Server starting on: http://0.0.0.0:3000")
	// TODO Make port configurable
	err = http.ListenAndServe(":3000", nil)
	if err != nil {
		fmt.Println(err.Error())
	}
}

package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

//go:embed index.html
var index string
var path string
var log *zap.Logger
var sizeLimitBytes int64

func internalServerErr(w http.ResponseWriter, err error) {
	log.Error(err.Error())
	http.Error(w, http.StatusText(http.StatusInternalServerError)+": "+err.Error(), http.StatusInternalServerError)
}

func writeToDisk(filename string, content []byte) error {
	filename = filepath.Join(path, filename)

	// Open and truncate or create file
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close() // nolint:errcheck

	// Write file
	_, err = file.Write(content)
	if err != nil {
		return fmt.Errorf("writing to file: %w", err)
	}

	return nil
}

func uploadBody(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		log.Error("Bad request, empty body")
		http.Error(w, http.StatusText(http.StatusBadRequest)+": Empty body", http.StatusBadRequest)
		return
	}

	// Wrap normal io.ReadCloser in MaxBytesReader
	r.Body = http.MaxBytesReader(w, r.Body, sizeLimitBytes)

	filename := r.PathValue("file")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(err.Error())
		var maxBytesError *http.MaxBytesError
		if errors.Is(err, maxBytesError) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		http.Error(w, http.StatusText(http.StatusBadRequest)+": "+err.Error(), http.StatusBadRequest)
		return
	}

	err = writeToDisk(filename, body)
	if err != nil {
		internalServerErr(w, err)
		return
	}

	log.Debug("Body content written to: " + filename)

	w.WriteHeader(http.StatusNoContent)
}

func uploadForm(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(sizeLimitBytes)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, http.ErrContentLength.Error()+": "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get reqFile form fields
	reqFile, handler, err := r.FormFile("file")
	if err != nil {
		log.Error(err.Error())
		http.Error(w, http.ErrMissingFile.Error()+": "+err.Error(), http.StatusBadRequest)
		return
	}
	defer reqFile.Close() // nolint:errcheck

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
		internalServerErr(w, err)
		return
	}

	log.Debug("Form content written to: " + handler.Filename)

	w.WriteHeader(http.StatusNoContent)
}

func listFiles(w http.ResponseWriter, r *http.Request) {
	accept := strings.Split(r.Header.Get("Accept"), ",")[0]
	log.Debug("Accept Header:" + accept)
	isJson := false
	switch accept {
	case "text/html": // Browser
		http.ServeFile(w, r, path)
	case "application/json": // Requested
		fallthrough
	case "text/json":
		isJson = true
		fallthrough
	default: // */* Curl and anything else
		entries, err := os.ReadDir(path)
		if err != nil {
			internalServerErr(w, err)
			return
		}

		if !isJson {
			for _, entry := range entries {
				_, err := fmt.Fprintln(w, entry.Name())
				if err != nil {
					internalServerErr(w, err)
					return
				}
			}
			return
		}

		var files []string
		for _, entry := range entries {
			files = append(files, entry.Name())
		}

		json, err := json.Marshal(files)
		if err != nil {
			internalServerErr(w, err)
			return
		}

		_, err = fmt.Fprint(w, string(json))
		if err != nil {
			internalServerErr(w, err)
			return
		}
	}
}

func getFile(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	filepath := filepath.Join(path, file)

	if file == "index.html" {
		_, err := fmt.Fprint(w, index)
		if err != nil {
			internalServerErr(w, err)
			return
		}
		return
	}

	log.Debug("Serving " + filepath)
	http.ServeFile(w, r, filepath)
}

func deleteFile(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	filepath := filepath.Join(path, file)
	err := os.RemoveAll(filepath)
	if err != nil {
		internalServerErr(w, fmt.Errorf("failed to delete file: %w", err))
		return
	}
	log.Debug("Deleted " + file)
}

func logger(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := log.With(
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)
		log.Debug("Response received")
		defer log.Debug("Response served")

		h.ServeHTTP(w, r)
	})
}

func debugStart() error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	log.Sugar().Debugf("%v %v %v %v", stat.Mode(), stat.Size(), stat.ModTime(), stat.Name())

	// Validate write permissions at startup
	testFile := "__TEST_FILE__"
	err = writeToDisk(testFile, []byte("test"))
	if err != nil {
		return err
	}
	err = os.RemoveAll(filepath.Join(path, testFile))
	if err != nil {
		return err
	}

	return nil
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
	log = log.Named("Glob")
	defer log.Sync() // nolint:errcheck

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	path = os.Getenv("GLOB_PATH")
	if path == "" {
		path = filepath.Join(".", "globs")
	}

	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Fatal(err.Error())
	}

	if dbg != "" {
		err := debugStart()
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	sizeLimitMbEnvVar := os.Getenv("SIZE_LIMIT_MB")
	if sizeLimitMbEnvVar == "" {
		sizeLimitBytes = 10 << 20 // Default 10Mi
	} else {
		sizeLimitMb, err := strconv.Atoi(sizeLimitMbEnvVar)
		if err != nil {
			log.Fatal(err.Error())
		}
		sizeLimitBytes = int64(sizeLimitMb) << 20
	}

	// GET/PUT/POST/DELETE
	// POST behaves like a PUT, most don't care about the difference and supports forms
	mux := http.NewServeMux()
	mux.Handle("GET /", logger(listFiles))
	mux.Handle("GET /{file}", logger(getFile))
	mux.Handle("PUT /", logger(uploadForm))
	mux.Handle("PUT /{file}", logger(uploadBody))
	mux.Handle("POST /", logger(uploadForm))
	mux.Handle("POST /{file}", logger(uploadBody))
	mux.Handle("DELETE /{file}", logger(deleteFile))

	server := http.Server{
		Addr:           ":" + port,
		Handler:        mux,
		MaxHeaderBytes: int(sizeLimitBytes),
	}

	log.Info("Listening on: http://0.0.0.0:" + port)

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err.Error())
	}
}

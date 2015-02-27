package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Minute

// letters is a list of allowed characters when generating a pipr token
var letters = []rune("abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRTUVWXYZ12346789")

type SourceRequest struct {
	rw   http.ResponseWriter
	req  *http.Request
	done chan bool
}

// sourceSinkMap keeps track of which pipr token goes to which sink pipr client
var sourceSinkMap = make(map[string]chan *SourceRequest)

// Host github homepage via proxying
var ghUrl, _ = url.Parse("https://github.com/mattwilliamson/webpipr/")
var ghProxy = http.ProxyURL(ghUrl)

// newToken generates a string of random characters of a given length
func newToken(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// getTokenExt parses out token and file extension
func getTokenExt(url string) (string, string) {
	tokenAndExt := strings.TrimSuffix(url, "/")
	tokenAndExt = tokenAndExt[strings.LastIndex(tokenAndExt, "/"):]
	tokenAndExt = strings.TrimPrefix(tokenAndExt, "/")

	// Figure out if there's a file extension and extract it from the token
	extSep := strings.LastIndex(tokenAndExt, ".")
	token := tokenAndExt
	ext := ".txt"

	// Found a ., get the extension
	if extSep != -1 {
		ext = tokenAndExt[extSep:]
		token = tokenAndExt[0:extSep]
	}

	ext = "." + strings.Trim(ext, ".")

	return token, ext
}

func typeForExt(ext string) string {
	t := mime.TypeByExtension(ext)
	if t == "" {
		return "text/plain"
	}
	return t
}

// index hosts the github page by proxy
func indexHandler(rw http.ResponseWriter, req *http.Request) {
	ghProxy(req)
}

// newToken redirects to a new in pipr with a fresh random token
func newHandler(rw http.ResponseWriter, req *http.Request) {
	url := "/out/" + newToken(16)
	http.Redirect(rw, req, url, http.StatusTemporaryRedirect)
}

// sink handles requests for clients waiting for callbacks
// if the token ends in .txt, we will format the POST/GET params like so: $key=$value\n
// if the token ends in .json, we will format the POST/GET params like so: {$key=$value, $key2=$value2}
func sinkHandler(outRw http.ResponseWriter, outReq *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in sink", r)
		}
	}()

	timeout := defaultTimeout
	token, ext := getTokenExt(outReq.URL.Path)

	// No token found... Get a new URL
	if token == "" {
		http.Redirect(outRw, outReq, "/", http.StatusTemporaryRedirect)
	}

	log.Printf("[o] START - SINKING for token '%v' with extension '%v'", token, ext)

	// Make a channel for the source request to send to
	sourceSinkMap[token] = make(chan *SourceRequest)

	select {
	case sourceReq := <-sourceSinkMap[token]:
		// Alllow the source request to return
		defer func() { sourceReq.done <- true }()

		// Set header based on extension
		outRw.Header().Add("Content-Type", typeForExt(ext))

		// Write response from sink to source (response)
		bytesWritten, err := io.Copy(sourceReq.rw, outReq.Body)
		log.Printf("[o] for %v wrote %v bytes to ->, err: %v", token, bytesWritten, err)

		// Write response from source to sink (request)
		sourceReq.req.ParseForm()

		// Convert to map[string]string instead of map[string][]string
		params := make(map[string]string)
		for key, values := range sourceReq.req.Form {
			params[key] = values[0]
		}

		if ext == ".json" {
			// Convert to json string
			reqParams, err := json.MarshalIndent(params, "", "    ")

			if err != nil {
				log.Printf("[o] error serializing json for '%v': %v", token, err)
			}

			// Write json to client
			fmt.Fprintf(outRw, string(reqParams)+"\n")
		} else {
			// Write plain text to client
			for k, v := range params {
				fmt.Fprintln(outRw, fmt.Sprintf("%v=%v", k, v))
			}
			bytesWritten, err = io.Copy(outRw, sourceReq.req.Body)
			log.Printf("-> for %v wrote %v bytes to [o], err: %v", token, bytesWritten, err)
		}
	case <-time.After(timeout):
		log.Printf("[o] timed out waiting for source '%v'", token)
	}

	delete(sourceSinkMap, token)

	log.Printf("[o] STOP - SINKING for token '%v' with extension '%v'", token, ext)
}

// source handles requests for clients sending callbacks to other waiting clients
func sourceHandler(rw http.ResponseWriter, req *http.Request) {
	token, ext := getTokenExt(req.URL.Path)

	log.Printf("-> START - SOURCING for token '%v' with extension '%v'", token, ext)

	// Find sync registered for this token
	sinkChan, ok := sourceSinkMap[token]

	if ok {
		log.Printf("-> found subscribers found for token '%v'", token)

		// Set header based on extension
		rw.Header().Add("Content-Type", typeForExt(ext))

		doneChan := make(chan bool)
		srcReq := SourceRequest{rw, req, doneChan}
		sinkChan <- &srcReq

		log.Printf("-> waiting for subscribers to finish for token '%v'", token)

		select {
		case <-doneChan:
			log.Printf("-> sent response for token '%v'", token)
		case <-time.After(defaultTimeout):
			log.Printf("-> timed out for token '%v'", token)
		}
	} else {
		log.Printf("-> no subscribers found for token '%v'", token)
		http.NotFound(rw, req)
	}

	log.Printf("-> STOP - SOURCING for token '%v' with extension '%v'", token, ext)
}

func main() {
	address := os.Getenv("WEBPIPR_ADDRESS")
	if address == "" {
		address = ":8080"
	}

	http.HandleFunc("/in/", sourceHandler)
	http.HandleFunc("/out/", sinkHandler)
	http.HandleFunc("/new/", newHandler)
	http.HandleFunc("/", indexHandler)

	log.Printf("Server running on %v...", address)

	err := http.ListenAndServe(address, nil)

	log.Fatalf("Error listening: %v", err)
}

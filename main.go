package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mholt/certmagic"
)

var locker sync.RWMutex
var domainMappings = map[string]string{}

const shop440Path = "https://api.shop440.com/sudo/domain_mappings"

func reloadMapping() {
	resp, err := http.Get(shop440Path)
	if err != nil {
		log.Println(err, "unable to query shop440 endpoint")
	}
	defer resp.Body.Close()

	responseData := struct {
		Data map[string]string `json:"data"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		log.Println(err, "unable to decode domain mapping")
	}

	locker.Lock()
	domainMappings = responseData.Data
	delete(domainMappings, "")
	domainMappings["books.localhost"] = "books"
	locker.Unlock()
	fmt.Println(domainMappings)
}

func main() {

	go func() {
		reloadMapping()
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			reloadMapping()
		}
	}()

	certmagic.Default.OnDemand = &certmagic.OnDemandConfig{
		DecisionFunc: func(name string) error {
			if name == "books.localhost" {
				return nil
			}
			locker.RLock()
			if _, ok := domainMappings[name]; ok {
				return nil
			}
			locker.RUnlock()

			return fmt.Errorf("domain %snot allowed", name)
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// log.Printf("request %#v \n", r.URL)
		// log.Printf("headers %#v \n", r.Header)

		hostname := r.URL.Hostname()
		var err error

		if hostname == "" {
			hostname = r.Host
		}
		if hostname == "" {
			hostURL, err := url.Parse(r.Header.Get("Referer"))
			if err != nil {
				log.Printf("unknow hosturl:%#v", hostURL)
				w.Write([]byte("unknown hostname"))
				return
			}
			hostname = hostURL.Hostname()
		}

		locker.RLock()
		shopID := domainMappings[hostname]
		locker.RUnlock()

		urlpath := r.URL.Path
		items := strings.Split(urlpath, "/")
		lastSegment := items[len(items)-1]
		if !strings.Contains(lastSegment, ".") {
			urlpath += "index.html"
		}
		rootPath := "https://storage.googleapis.com/shop440customsites/"
		finalPath := rootPath + shopID + urlpath
		fmt.Println(finalPath)

		resp, err := http.Get(finalPath)
		if err != nil {
			log.Println("ERROR:", err.Error())
		}
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			log.Println("ERROR:", err.Error())
		}

		for k, v := range resp.Header {
			w.Header().Set(k, v[0])
		}

		// w.WriteHeader(resp.StatusCode)

	})

	err := certmagic.HTTPS([]string{}, mux)
	if err != nil {
		log.Panic(err)
	}
	// log.Fatal(http.ListenAndServe(":80", mux))
}

/*
addEventListener('fetch', event => {
  event.respondWith(handleRequest(event.request))
})

/**
 * Fetch and log a request
 * @param {Request} request
async function handleRequest(request) {
  const parsedUrl = new URL(request.url)
  const hostname = parsedUrl.hostname
  const subdomain = hostname.substr(0, hostname.indexOf('.'))
  if (["merchants", "marketplace", "api","shop440"].includes(subdomain) ){
    return fetch(request)
  }

  let path = parsedUrl.pathname

  let lastSegment = path.substring(path.lastIndexOf('/'))
  if (lastSegment.indexOf('.') === -1) {
    path += 'index.html'
  }

  const finalPath = "https://storage.googleapis.com/shop440customsites/"+subdomain + path
  return fetch(finalPath)
}
*/

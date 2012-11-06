/*
	Written by José Carlos Nieto <xiam@menteslibres.org>
	License MIT
*/

package proxy

import (
	"github.com/xiam/hyperfox/mimext"
	"io"
	"log"
	"net/http"
	"time"
	"os"
	"fmt"
	"path"
	"strings"
)

/*
	Returns a io.WriteCloser that will be called
	everytime new content is received from the destination.

	Writer functions should not edit response headers or
	body.
*/
type Writer func(*http.Response) io.WriteCloser

/*
	Called before giving any output to the client.

	Director functions can be used to edit response headers
	and body before arriving to the client.
*/
type Director func(*http.Response) error

/*
	Called right before sending content to the client.

	Logger functions should not edit response headers or
	body.
*/
type Logger func(*http.Response) error

/*
	Storage directories.
*/
var ArchiveDir = "archive"
var ClientDir = "client"

const PS = string(os.PathSeparator)

/*
	Proxy.
*/
type Proxy struct {
	srv       http.Server
	Bind      string
	Writers   []Writer
	Directors []Director
	Loggers   []Logger
}

/*
	Returns a new Proxy.
*/
func New() *Proxy {
	self := &Proxy{}
	self.Writers = []Writer{}
	self.Directors = []Director{}
	self.Bind = "0.0.0.0:9999"
	return self
}

/*
	Adds a Writer function to the Proxy.

	Writer functions are called in the same order
	they are added.
*/
func (self *Proxy) AddWriter(wri Writer) {
	self.Writers = append(self.Writers, wri)
}

/*
	Adds a Director function to the Proxy.

	Director functions are called in the same order
	they are added.
*/
func (self *Proxy) AddDirector(dir Director) {
	self.Directors = append(self.Directors, dir)
}

/*
	Adds a Logger function to the Proxy.

	Logger functions are called in the same order
	they are added.
*/
func (self *Proxy) AddLogger(dir Logger) {
	self.Loggers = append(self.Loggers, dir)
}

// http://golang.org/src/pkg/net/http/httputil/reverseproxy.go#L72
func copyHeader(dst http.Header, src http.Header) {
	for k, _ := range dst {
		dst.Del(k)
	}
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

/*
	Catches a client request and proxies it to the
	destination server.

	Should not be called directly.
*/
func (self *Proxy) ServeHTTP(wri http.ResponseWriter, req *http.Request) {

	out := new(http.Request)

	transport := http.DefaultTransport

	*out = *req
	out.Proto = "HTTP/1.1"
	out.ProtoMajor = 1
	out.ProtoMinor = 1
	out.Close = false

	out.URL.Scheme = "http"
	out.URL.Host = req.Host

	out.Header.Add("Host", req.Host)

	res, err := transport.RoundTrip(out)

	if err != nil {
		panic(err)
	}

	self.intercept(wri, res)
}

/*
	Returns an appropriate name for a file that needs to be associated
	with a response.
*/
func ArchiveFile(res *http.Response) string {

	contentType := res.Header.Get("Content-Type")

	file := strings.Trim(res.Request.URL.Path, "/")

	if file == "" {
		file = "index"
	}

	if path.Ext(file) == "" {
		file = file + "." + mimext.Ext(contentType)
	}

	if res.Header.Get("Content-Encoding") == "gzip" {
		file = file + ".gz"
	}

	file = ArchiveDir + PS + res.Request.URL.Host + PS + file

	return file
}

func now() string {
	t := time.Now().Local()
	name := fmt.Sprintf(
		"%04d%02d%02d-%02d%02d%02d-%09d",
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
		t.Nanosecond(),
	)
	return name + ".bin"
}

/*
	Returns an appropriate name for a file that needs to be associated
	with a request.
*/
func ClientFile(res *http.Response) string {

	file := strings.Trim(res.Request.URL.Path, "/")

	if file == "" {
		file = "index"
	}

	clientAddr := strings.SplitN(res.Request.RemoteAddr, ":", 2)

	file = ClientDir + PS + clientAddr[0] + PS + res.Request.URL.Host + PS + file + PS + now()

	return file
}

func Workdir(dir string) error {
	return os.MkdirAll(dir, os.ModeDir|os.FileMode(0755))
}

/*
	Catches a server response and processes it before sending it
	to the client.
*/
func (self *Proxy) intercept(dst http.ResponseWriter, res *http.Response) {
	var i int

	/* Applying directors before copying headers. */
	for i, _ = range self.Directors {
		self.Directors[i](res)
	}

	/* Copying headers. */
	copyHeader(dst.Header(), res.Header)

	/* Writing status. */
	dst.WriteHeader(res.StatusCode)

	wclosers := []io.WriteCloser{}

	/* Handling requests. */
	for i, _ := range self.Writers {
		wcloser := self.Writers[i](res)
		if wcloser != nil {
			wclosers = append(wclosers, wcloser)
		}
	}

	/* Applying loggers */
	for i, _ = range self.Loggers {
		self.Loggers[i](res)
	}

	/* Writing response. */
	if res.Body != nil {
		writers := []io.Writer{dst}
		for i, _ := range wclosers {
			writers = append(writers, wclosers[i])
		}
		io.Copy(io.MultiWriter(writers...), res.Body)
	}

	/* Closing */
	res.Body.Close()

	for i, _ := range wclosers {
		wclosers[i].Close()
	}

}

/*
	Starts a web server.
*/
func (self *Proxy) Start() error {

	self.srv = http.Server{
		Addr:    self.Bind,
		Handler: self,
	}

	log.Printf("Hyperfox is ready at %s\n", self.Bind)

	err := self.srv.ListenAndServe()

	if err != nil {
		log.Printf("Failed to bind.\n")
	}

	return err
}

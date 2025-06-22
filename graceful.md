## [Blog](https://marcofranssen.nl/blog).

# Improved graceful shutdown webserver

![Marco Franssen](https://marcofranssen.nl/_next/image?url=%2Fimages%2Fprofile.jpg&w=96&q=75)

Marco Franssen / July 4, 2019

8 min read • 1534 words

![Cover Image for Improved graceful shutdown webserver](https://marcofranssen.nl/_next/image?url=%2Fimages%2F933faf01ec92c3ab47275d096d3e8c0f2e0d87e5.png&w=3840&q=75)

In a [previous blogpost](https://marcofranssen.nl/go-webserver-with-graceful-shutdown "Go Webserver with graceful shutdown") I wrote how to create a Webserver in Go with graceful shutdown. This time I want to show you a more improved version which you can utilize better in your projects as it can be used as a drop in `server.go` file in your project where I also make use of some popular high performing libraries.

In previous example I coded the full example in main.go. Although nothing wrong with that I learned while building microservices for a while it would be more convenient for me if I could copy my boilerplate more easy in the new project and keep my main.go more clean. As most of the microservices have different kind of bootstrap I use main.go now solely for bootstrapping different packages and components like the webserver.

We would be talking about a very plain main.go where I would just delegate all the work to other packages and files and solely use it to bootstap my application. See following as an example.

main.go

```
package main

import (
	"flag"
)

var (
	listenAddr string
)

func main() {
	flag.StringVar(&listenAddr, "listen-addr", ":5000", "server listen address")
	flag.Parse()

	// bootstrap any other packages or components here.
	// things like database connectivity or domain services
	// you can inject this as a dependency on your webserver if required
	// by adding another parameter to the NewServer func.
	// Now I left this for brevity.
	server, err := NewServer(listenAddr)
	if err != nil {
		panic(err)
	}
	server.Start()
}

```

For new readers of my blog and readers new to Go, I would like to refer you to [following blog post](https://marcofranssen.nl/start-on-your-first-golang-project "Start on your first Golang project"). In [this blog](https://marcofranssen.nl/start-on-your-first-golang-project "Start on your first Golang project") I explain you how to setup a development environment for Go and how to initialize a Go project. For readers already known to Go please continue reading.

Now lets have a look on how I would implement my web server in a file named server.go, which I will be able to drop into any new project as a boilerplate. To mention first I would like to highlight you don't necesarely need any third party libraries for very simple webservers. However for convenience and development speed I took 2 external dependencies which are outstanding in benchmarks and have a good community supporting the library.

First of these libraries is **chi** which is a nice and very well performing library to build webservers and restful APIs. It ships with middelwares and allows you to easily build your own middleware. More on that further on where I will show you as well a zap.Logger middleware I created to use the zap.Logger for all requests in a chi middleware.

That brings us to the second library I included. The **zap.Logger**, created by Uber. It is a nice very high performant logger using different logging levels and different logger backends allowing you to do nicely structured logging. For example logging into Elasticsearch and Kibana.

At the end of this blog I included the urls to both the libraries as further reference.

So lets start by first implementing our NewServer func which returns our server struct.

server.go

```
package main

// Server provides an http.Server
type Server struct {
  l *zap.Logger
  *http.Server
}

// NewServer creates and configures a server serving all application routes.
//
// The server implements a graceful shutdown and utilizes zap.Logger for logging purposes.
// chi.Mux is used for registering some convenient middlewares and easy configuration of
// routes using different http verbs.
func NewServer(listenAddr string) (*Server, error) {
  logger, err := zap.NewDevelopment(zap.AddStacktrace(zapcore.FatalLevel))
  if err != nil {
    log.Fatalf("Can't initialize zap logger: %v", err)
  }
  defer logger.Sync()

  logger.Info("Configure API")
  api := newAPI(logger)

  errorLog, _ := zap.NewStdLogAt(logger, zap.ErrorLevel)
  srv := http.Server{
    Addr:         listenAddr,
    Handler:      api,
    ErrorLog:     errorLog,
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  15 * time.Second,
  }

  return &Server{logger, &srv}, nil
}
```

As you can see we are creating a zap.Logger using the convenience factory func `NewDevelopment`. Zap does offer more complex and finegrained configuration of the logger, but in general I use this simple setup. Furthermore you see I'm calling a func `newAPI(logger)`. This I use to bootstrap a `chi.Mux` that allows us to define the endpoints for our webserver. E.g. RESTFULL apis etc. Below the implementation of this method.

server.go

```
func newAPI(logger *zap.Logger) *chi.Mux {
  r := chi.NewRouter()

  r.Use(middleware.RequestID)
  r.Use(zapLogger(logger))
  r.Use(middleware.Recoverer)

  r.Get("/", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
  })
  r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("pong"))
  })

  // register more routes over here...

  logRoutes(r, logger)

  return r
}
```

In here you see we are registering one url which will respond when we make a GET request with a HTTP status OK (200). Furthermore you see I'm registering some middleware which ships with **chi**. You also see I'm using a zapLogger middleware, which I created myself to be able to use zap.Logger as a middleware in **chi**. Last but not least I added some convenient debugging to log the registered routes on DEBUG level. This is very helpfull when defining bigger APIs with many routes and HTTP verbs.

Below you see the implementation of my zapLogger middleware. It utilizes the RequestID middleware as well to print this in the log as well. I'm also using a nice little trick using defer which allows us to calculate the latency in the request. Defer will invoke only after next.ServeHTTP(ww, r) is called, which goes further down the middleware stack and eventually into your endpoint code. This makes the logger log the latency of all the code which is invoked in between just before it returns the response to the consumer of your webserver. You can read more on the use of defer in Go in [this blog post](https://marcofranssen.nl/the-use-of-defer-in-go "The use of defer in Go").

server.go

```
func zapLogger(l *zap.Logger) func(next http.Handler) http.Handler {
  return func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

      t1 := time.Now()
      defer func() {
        l.Info("Served",
          zap.String("proto", r.Proto),
          zap.String("path", r.URL.Path),
          zap.Duration("lat", time.Since(t1)),
          zap.Int("status", ww.Status()),
          zap.Int("size", ww.BytesWritten()),
          zap.String("reqId", middleware.GetReqID(r.Context())),
        )
      }()

      next.ServeHTTP(ww, r)
    })
  }
}

func logRoutes(r *chi.Mux, logger *zap.Logger) {
  if err := chi.Walk(r, zapPrintRoute(logger)); err != nil {
    logger.Error("Failed to walk routes:", zap.Error(err))
  }
}

func zapPrintRoute(logger *zap.Logger) chi.WalkFunc {
  return func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
    route = strings.Replace(route, "/*/", "/", -1)
    logger.Debug("Registering route", zap.String("method", method), zap.String("route", route))
    return nil
  }
}
```

Above you also see the logRoutes func that utilizes chi.Walk to walk all the different routes and log them using our zap.Logger on DEBUG level. This will help you to debug issues like what are the exact endpoints in more complex routers.

Last but not least lets have a look on the graceful shutdown part which I also slightly simplified compared to my previous example.

server.go

```
// Start runs ListenAndServe on the http.Server with graceful shutdown
func (srv *Server) Start() {
  srv.l.Info("Starting server...")
  defer srv.l.Sync()

  go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
      srv.l.Fatal("Could not listen on", zap.String("addr", srv.Addr), zap.Error(err))
    }
  }()
  srv.l.Info("Server is ready to handle requests", zap.String("addr", srv.Addr))
  srv.gracefulShutdown()
}

func (srv *Server) gracefulShutdown() {
  quit := make(chan os.Signal, 1)

  signal.Notify(quit, os.Interrupt)
  sig := <-quit
  srv.l.Info("Server is shutting down", zap.String("reason", sig.String()))

  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer cancel()

  srv.SetKeepAlivesEnabled(false)
  if err := srv.Shutdown(ctx); err != nil {
    srv.l.Fatal("Could not gracefully shutdown the server", zap.Error(err))
  }
  srv.l.Info("Server stopped")
}
```

As you might see we are using a subroutine to run the webserver so it runs in the background and then we use a Signal channel to wait for a ctrl+c signal to gracefully shutdown the server.

## TL;DR

Finally we came to the end. Below the full server.go file.

main.go

```
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Server provides an http.Server
type Server struct {
	l *zap.Logger
	*http.Server
}

// NewServer creates and configures a server serving all application routes.
//
// The server implements a graceful shutdown and utilizes zap.Logger for logging purposes.
// chi.Mux is used for registering some convenient middlewares and easy configuration of
// routes using different http verbs.
func NewServer(listenAddr string) (*Server, error) {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zapcore.FatalLevel))
	if err != nil {
		log.Fatalf("Can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Configure API")
	api := newAPI(logger)

	errorLog, _ := zap.NewStdLogAt(logger, zap.ErrorLevel)
	srv := http.Server{
		Addr:         listenAddr,
		Handler:      api,
		ErrorLog:     errorLog,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return &Server{logger, &srv}, nil
}

// Start runs ListenAndServe on the http.Server with graceful shutdown
func (srv *Server) Start() {
	srv.l.Info("Starting server...")
	defer srv.l.Sync()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srv.l.Fatal("Could not listen on", zap.String("addr", srv.Addr), zap.Error(err))
		}
	}()
	srv.l.Info("Server is ready to handle requests", zap.String("addr", srv.Addr))
	srv.gracefulShutdown()
}

func newAPI(logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(zapLogger(logger))
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	// register more routes over here...

	logRoutes(r, logger)

	return r
}

func zapLogger(l *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			t1 := time.Now()
			defer func() {
				l.Info("Served",
					zap.String("proto", r.Proto),
					zap.String("path", r.URL.Path),
					zap.Duration("lat", time.Since(t1)),
					zap.Int("status", ww.Status()),
					zap.Int("size", ww.BytesWritten()),
					zap.String("reqId", middleware.GetReqID(r.Context())),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func logRoutes(r *chi.Mux, logger *zap.Logger) {
	if err := chi.Walk(r, zapPrintRoute(logger)); err != nil {
		logger.Error("Failed to walk routes:", zap.Error(err))
	}
}

func zapPrintRoute(logger *zap.Logger) chi.WalkFunc {
	return func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		route = strings.Replace(route, "/*/", "/", -1)
		logger.Debug("Registering route", zap.String("method", method), zap.String("route", route))
		return nil
	}
}

func (srv *Server) gracefulShutdown() {
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, os.Interrupt)
	sig := <-quit
	srv.l.Info("Server is shutting down", zap.String("reason", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv.SetKeepAlivesEnabled(false)
	if err := srv.Shutdown(ctx); err != nil {
		srv.l.Fatal("Could not gracefuly shutdown the server", zap.Error(err))
	}
	srv.l.Info("Server stopped")
}

```

You should be able to build and run your webserver now as following, assuming you created the main.go and server.go files. Ofcourse you also initialized your go module.

terminal

```
$ go build . && ./improved-graceful-webserver.exe
2019-07-04T23:04:55.775+0200    INFO    improved-graceful-webserver/server.go:37        Configure API
2019-07-04T23:04:55.775+0200    DEBUG   improved-graceful-webserver/server.go:119       Registering route       {"method": "GET", "route": "/"}
2019-07-04T23:04:55.776+0200    DEBUG   improved-graceful-webserver/server.go:119       Registering route       {"method": "GET", "route": "/ping"}
2019-07-04T23:04:55.776+0200    INFO    improved-graceful-webserver/server.go:55        Starting server...
2019-07-04T23:04:55.798+0200    INFO    improved-graceful-webserver/server.go:63        Server is ready to handle requests      {"addr": ":5000"}
2019-07-04T23:05:10.371+0200    INFO    improved-graceful-webserver/server.go:95        Served  {"proto": "HTTP/1.1", "path": "/", "lat": "0s", "status": 200, "size": 0, "reqId": "DESKTOP-B249DU4/DBQUBZ6RNh-000001"}
2019-07-04T23:05:15.187+0200    INFO    improved-graceful-webserver/server.go:95        Served  {"proto": "HTTP/1.1", "path": "/ping", "lat": "0s", "status": 200, "size": 4, "reqId": "DESKTOP-B249DU4/DBQUBZ6RNh-000002"}
2019-07-04T23:05:20.795+0200    INFO    improved-graceful-webserver/server.go:129       Server is shutting down {"reason": "interrupt"}
2019-07-04T23:05:20.797+0200    INFO    improved-graceful-webserver/server.go:138       Server stopped
```

## References

As promised I would include some links to the Go documentation of **chi** and **zap**. Furthermore I included a zip file with the full example which you can download and run on your local. For the folks not having golang on their machine you can also use my embedded Docker and docker-compose file to run it using the following:

```
docker-compose up --build
```

- [Go doc chi](https://godoc.org/github.com/go-chi/chi)
- [Go doc zap](https://godoc.org/go.uber.org/zap)
- [Download full zip](https://marcofranssen.nl/downloads/code/improved-graceful-webserver.zip "Download improved graceful webserver")

Many thanks if you managed to make it to the end of this blog post. Please leave me some feedback and share this blog post with your friends and colleagues. Want to read more on Go also check [my other blogs on Go](https://marcofranssen.nl/categories/golang "The use of defer in Go").

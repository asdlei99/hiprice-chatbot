package main

import (
  "fmt"
  "io/ioutil"
  "net/http"

  "github.com/kwf2030/commons/conv"
  "github.com/rs/cors"
  "strings"
)

var favicon []byte

var hostPort string

func sendResp(w http.ResponseWriter, ret int, status string, data interface{}) {
  m := make(map[string]interface{}, 3)
  m["ret"] = ret
  if status != "" {
    m["status"] = status
  }
  if data != nil {
    m["data"] = data
  }
  res, _ := conv.MapToJSON(m)
  if res == nil {
    w.WriteHeader(http.StatusInternalServerError)
  } else {
    w.Write(res)
  }
}

// 不支持跨域
// GET  /admin
// GET  /admin/static/
// GET  /admin/api/bots
// POST /admin/api/bot
// GET  /admin/api/bot?uuid=xx
// DEL  /admin/api/bot?uin=xx
func adminMux() *http.ServeMux {
  ret := http.NewServeMux()
  ret.HandleFunc("/", http.NotFound)
  ret.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    data, e := ioutil.ReadFile("admin/index.html")
    if e != nil {
      logger.Error().Err(e).Msg("ERR: ReadFile")
      w.WriteHeader(http.StatusInternalServerError)
      return
    }
    w.Write(data)
  })
  ret.HandleFunc("/admin/static/", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, r.URL.Path[1:])
  })
  ret.HandleFunc("/admin/api/bots", botsHandler)
  ret.HandleFunc("/admin/api/bot", botHandler)
  return ret
}

// 支持跨域
// GET  /web/api/watchlist?u=xx
// POST /web/api/unwatch?u=xx
// POST /web/api/remind?u=xx
// POST /web/api/settings?u=xx
func webMux() *http.ServeMux {
  ret := http.NewServeMux()
  ret.HandleFunc("/", http.NotFound)
  ret.HandleFunc("/web/api/watchlist", watchListHandler)
  ret.HandleFunc("/web/api/unwatch", unwatchHandler)
  ret.HandleFunc("/web/api/remind", remindHandler)
  ret.HandleFunc("/web/api/settings", settingsHandler)
  return ret
}

func launchServer() {
  cs := cors.AllowAll()
  admin := adminMux()
  web := webMux()
  handler := func(w http.ResponseWriter, r *http.Request) {
    p := r.URL.Path
    if p == "/favicon.ico" {
      w.Header().Set("Content-Type", "image/x-icon")
      if len(favicon) == 0 {
        favicon, _ = ioutil.ReadFile("admin/static/favicon.ico")
      }
      w.Write(favicon)
      return
    }
    if len(p) >= 6 && p[:6] == "/admin" {
      a1, a2, ok := r.BasicAuth()
      if !ok || a1 != Conf.Server.User || a2 != Conf.Server.Password {
        w.Header().Set("WWW-Authenticate", "Basic realm=\"Login\"")
        w.WriteHeader(http.StatusUnauthorized)
        return
      }
      h, _ := admin.Handler(r)
      h.ServeHTTP(w, r)
      return
    }
    h, _ := web.Handler(r)
    cs.Handler(h).ServeHTTP(w, r)
  }
  server := &http.Server{
    Addr:    fmt.Sprintf("%s:%d", Conf.Server.Host, Conf.Server.Port),
    Handler: http.HandlerFunc(handler),
  }
  var e error
  if Conf.Server.Cert != "" && Conf.Server.Key != "" {
    e = server.ListenAndServeTLS(Conf.Server.Cert, Conf.Server.Key)
  } else {
    e = server.ListenAndServe()
  }
  if e != nil {
    panic(e)
  }
  logger.Info().Msg("server started, listening on " + server.Addr)
}

func redirectHTTP() {
  if Conf.Server.Cert == "" || Conf.Server.Key == "" {
    return
  }
  e := http.ListenAndServe(fmt.Sprintf("%s:80", Conf.Server.Host), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
      w.WriteHeader(http.StatusBadRequest)
      return
    }
    if hostPort == "" {
      i := strings.Index(r.Host, ":")
      if i == -1 {
        hostPort = fmt.Sprintf("%s:%d", r.Host, Conf.Server.Port)
      } else {
        hostPort = fmt.Sprintf("%s:%d", r.Host[:i], Conf.Server.Port)
      }
    }
    r.URL.Host = hostPort
    r.URL.Scheme = "https"
    http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
  }))
  if e != nil {
    panic(e)
  }
}

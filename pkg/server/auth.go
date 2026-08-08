package server

import (
	"net/http"
	"strconv"
	"time"

	"lan-share/pkg/token"
)

const tokenParam = "token"
const expParam = "exp"

func (s *Server) checkToken(r *http.Request) bool {
	if s.secret == "" {
		return true
	}
	q := r.URL.Query()
	tok := q.Get(tokenParam)
	expStr := q.Get(expParam)
	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	return token.Valid(s.secret, r.URL.Path, tok, time.Unix(expUnix, 0))
}

// LinkURL 为用户组装带 token 的直链（用于 main 打印）。
func (s *Server) LinkURL(ip string, port int, path string, expire time.Time) string {
	if s.secret == "" {
		return "http://" + ip + ":" + strconv.Itoa(port) + path
	}
	q := "?token=" + token.Generate(s.secret, path, expire) + "&exp=" + strconv.FormatInt(expire.Unix(), 10)
	return "http://" + ip + ":" + strconv.Itoa(port) + path + q
}

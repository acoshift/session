package session

import (
	"bytes"
	"encoding/gob"
	"net/http"
	"time"
)

type (
	markDestroy struct{}
	markRotate  struct{}
)

// Session type
type Session struct {
	id      string
	oldID   string // for rotate
	data    map[interface{}]interface{}
	mark    interface{}
	changed bool

	// cookie config
	Name     string
	Domain   string
	Path     string
	HTTPOnly bool
	MaxAge   time.Duration
	Secure   bool

	// disable
	DisableRenew bool
}

func init() {
	gob.Register(map[interface{}]interface{}{})
	gob.Register(timestampKey{})
}

// session internal data
type (
	timestampKey struct{}
)

func (s *Session) encode() []byte {
	if len(s.data) == 0 {
		return []byte{}
	}

	buf := bytes.Buffer{}
	err := gob.NewEncoder(&buf).Encode(s.data)
	if err != nil {
		// this should never happended
		// or developer don't register type into gob
		panic("session: can not encode data; " + err.Error())
	}
	return buf.Bytes()
}

func (s *Session) decode(b []byte) {
	s.data = make(map[interface{}]interface{})
	if len(b) > 0 {
		gob.NewDecoder(bytes.NewReader(b)).Decode(&s.data)
	}
}

func (s *Session) shouldRenew() bool {
	if s.DisableRenew {
		return false
	}
	sec, _ := s.Get(timestampKey{}).(int64)
	if sec < 0 {
		return false
	}
	if sec == 0 {
		// backward-compability
		return true
	}
	t := time.Unix(sec, 0)
	if time.Now().Sub(t) < s.MaxAge/2 {
		return false
	}
	return true
}

// Get gets data from session
func (s *Session) Get(key interface{}) interface{} {
	if s.data == nil {
		return nil
	}
	return s.data[key]
}

// Set sets data to session
func (s *Session) Set(key, value interface{}) {
	if s.data == nil {
		s.data = make(map[interface{}]interface{})
	}
	s.changed = true
	s.data[key] = value
}

// Del deletes data from session
func (s *Session) Del(key interface{}) {
	if s.data == nil {
		return
	}
	if _, ok := s.data[key]; ok {
		s.changed = true
		delete(s.data, key)
	}
}

// Rotate rotates session id
// use when change user access level to prevent session fixation
//
// can not use rotate and destory same time
func (s *Session) Rotate() {
	s.mark = markRotate{}
}

// Destroy destroys session from store
func (s *Session) Destroy() {
	s.mark = markDestroy{}
}

func (s *Session) setCookie(w http.ResponseWriter) {
	if _, ok := s.mark.(markDestroy); ok {
		http.SetCookie(w, &http.Cookie{
			Name:     s.Name,
			Domain:   s.Domain,
			Path:     s.Path,
			HttpOnly: s.HTTPOnly,
			Value:    "",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			Secure:   s.Secure,
		})
		return
	}

	if len(s.id) > 0 && s.shouldRenew() {
		s.Rotate()
	}

	// if session was modified, save session to store,
	// if not don't save to store to prevent store overflow
	if _, ok := s.mark.(markRotate); ok {
		s.oldID = s.id
		s.id = ""
	}

	if len(s.id) > 0 {
		return
	}

	s.id = generateID()
	http.SetCookie(w, &http.Cookie{
		Name:     s.Name,
		Domain:   s.Domain,
		Path:     s.Path,
		HttpOnly: s.HTTPOnly,
		Value:    s.id,
		MaxAge:   int(s.MaxAge / time.Second),
		Expires:  time.Now().Add(s.MaxAge),
		Secure:   s.Secure,
	})
}

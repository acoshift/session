package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"net/http"
	"time"
)

type (
	markSave    struct{}
	markDestroy struct{}
	markRotate  struct{}
)

// Session type
type Session struct {
	id          string
	oldID       string // for rotate
	data        map[interface{}]interface{}
	rawData     []byte
	mark        interface{}
	encodedData []byte

	// cookie config
	generateID func() string
	Name       string
	Domain     string
	Path       string
	HTTPOnly   bool
	MaxAge     time.Duration
	Secure     bool

	// disable
	DisableRenew bool
}

func init() {
	gob.Register(map[interface{}]interface{}{})
	gob.Register(timestampKey{})
}

type sessionKey struct{}

// session internal data
type (
	timestampKey struct{}
)

// Get gets session from context
func Get(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionKey{}).(*Session)
	return s
}

// Set sets session to context
func Set(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

func (s *Session) encode() ([]byte, error) {
	if len(s.data) == 0 {
		return []byte{}, nil
	}

	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(&s.data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	if sec <= 0 {
		return false
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
	s.data[key] = value
}

// Del deletes data from session
func (s *Session) Del(key interface{}) {
	if s.data == nil {
		return
	}
	delete(s.data, key)
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
			Secure:   s.Secure,
		})
		return
	}

	// set cookie only if session value changed
	var err error
	s.encodedData, err = s.encode()
	if err != nil {
		// this should never happended
		// or developer don't register struct into gob
		return
	}

	if s.shouldRenew() {
		s.Rotate()
	}

	if _, ok := s.mark.(markRotate); ok {
		s.oldID = s.id
		s.id = ""
	} else if bytes.Compare(s.rawData, s.encodedData) == 0 {
		if len(s.encodedData) == 0 {
			// empty session
			return
		}
	} else {
		s.mark = markSave{}
	}

	if len(s.id) > 0 {
		return
	}

	s.id = s.generateID()
	s.Set(timestampKey{}, time.Now().Unix())
	s.encodedData, _ = s.encode()
	http.SetCookie(w, &http.Cookie{
		Name:     s.Name,
		Domain:   s.Domain,
		Path:     s.Path,
		HttpOnly: s.HTTPOnly,
		Value:    s.id,
		MaxAge:   int(s.MaxAge / time.Second),
		Secure:   s.Secure,
	})
}

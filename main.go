package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/garyburd/redigo/redis"
	"strings"
)

const (
	myDomain = "http://localhost:8080/"
	TTLSec   = 60
)

type redisClient struct {
	redis.Conn
}

func newRedisClient() *redisClient {
	con, err := redis.Dial("tcp", ":6379")
	if err != nil {
		panic("cant connect to redis")
	}
	cl := redisClient{}
	con.Send("SELECT", 0)
	cl.Conn = con
	return &cl
}

func (c *redisClient) close() {
	c.Close()
}

func (c *redisClient) get(key string) (interface{}, error) {
	return c.Do("GET", key)
}

func (c *redisClient) set(key, val string) error {
	return c.Send("SETEX", key, TTLSec, val)
}

func shorten(w http.ResponseWriter, r *http.Request) {
	rd := newRedisClient()
	defer rd.close()
	c := rd

	requestURL := requestURL(r)
	paramRand := r.URL.Path

	val, err := c.get(paramRand)
	if err != nil {
		panic(fmt.Sprintf("cant get from redis: %s", paramRand))
	}
	shortID := func(val interface{}) string {
		if val != nil {
			// 重複あり
			return val.(string)
		}
		// 重複なし = URL生成
		shortID := random()
		c.set(shortID, requestURL)
		c.Flush()
		return shortID
	}(val)

	// debug message
	fmt.Fprintf(w, "DEBUG: create URL - Refer: %s, Transfer: %s\n", requestURL, myDomain+"s/"+shortID) // TODO: url .join
}

func random() string {
	var n uint64
	binary.Read(rand.Reader, binary.LittleEndian, &n)
	return strconv.FormatUint(n, 36)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	rd := newRedisClient()
	defer rd.close()
	c := rd

	requestURL := requestURL(r)
	paramRand := r.URL.Path

	paramShortID := strings.Replace(paramRand, "/s/", "", 1)
	ret, err := c.get(paramShortID)
	if err != nil {
		panic(fmt.Sprintf("cant get from redis: %s", paramShortID))
	}
	if ret == nil {
		fmt.Fprintf(w, "The URL %s does not seems registered as short URL.", requestURL)
		return
	}

	redirectURL := string(ret.([]byte))
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func requestURL(r *http.Request) string {
	return fmt.Sprintf("https://%s%s", r.Host, r.RequestURI) // TODO: url .join
}

func main() {
	// handles URL routing
	http.HandleFunc("/s/", redirect) // redirect
	http.HandleFunc("/", shorten)    // shorten
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

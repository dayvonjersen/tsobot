package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/fluffle/goirc/client"
)

// Configuration variables, passed in with command line flags
var (
	host      string
	port      int
	ssl       bool
	nick      string
	pass      string
	join      string
	admin     string
	cache_dir string
)

var irc *client.Conn

var sendMessage func(who, msg string)

func parseMessage(who, msg, nick string) {
	if !botCommandRe.MatchString(msg) {
		return
	}

	m := botCommandRe.FindStringSubmatch(msg)
	cmd, arg := m[1], m[2]

	if c, ok := botCommands[cmd]; ok {
		if !c.admin || (c.admin && isAdmin(nick)) {
			c.fn(who, arg, nick)
		} else {
			//log.Printf("%#v\n", botAdmins)
			sendMessage(nick, "Access denied.")
		}
	}
}

var sb *Scoreboard

func main() {
	flag.StringVar(
		&host,
		"host",
		"irc.rizon.net",
		"host",
	)
	flag.IntVar(
		&port,
		"port",
		6697,
		"port",
	)
	flag.BoolVar(
		&ssl,
		"ssl",
		true,
		"use ssl?",
	)

	flag.StringVar(
		&nick,
		"nick",
		"tsobot",
		"nick",
	)
	flag.StringVar(
		&pass,
		"pass",
		"",
		"NickServ IDENTIFY password (optional)",
	)
	flag.StringVar(
		&join,
		"join",
		"tso",
		"space separated list of channels to join",
	)

	flag.StringVar(
		&admin,
		"admin",
		"tso",
		"space separated list of privileged nicks",
	)
	flag.StringVar(
		&cache_dir,
		"cache_dir",
		".cache",
		"directory to cache datas like rss feeds",
	)

	flag.Parse()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, os.Kill)
	go func() {
		<-sig
		log.Println("we get signal")
		for _, ch := range strings.Split(join, " ") {
			irc.Part("#"+ch, "we get signal")
		}
		<-time.After(time.Second)
		irc.Quit()
		if sb != nil {
			sb.Save()
		}
		os.Exit(0)
	}()

	cfg := client.NewConfig(nick)
	if ssl {
		cfg.SSL = true
		cfg.SSLConfig = &tls.Config{ServerName: host, InsecureSkipVerify: true}
		cfg.NewNick = func(n string) string { return n + "^" }
	}
	cfg.Server = fmt.Sprintf("%s:%d", host, port)
	irc = client.Client(cfg)

	irc.HandleFunc(client.CONNECTED, func(c *client.Conn, l *client.Line) {
		if pass != "" {
			irc.Privmsg("NickServ", "IDENTIFY "+pass)
		}
		for _, ch := range strings.Split(join, " ") {
			irc.Join("#" + ch)
		}
		botAdmins = sort.StringSlice(strings.Split(admin, " "))
	})

	ded := make(chan struct{})
	irc.HandleFunc(client.DISCONNECTED, func(c *client.Conn, l *client.Line) {
		close(ded)
	})

	sendMessage = func(who, msg string) {
		irc.Privmsg(who, msg)
	}

	sb = newScoreboard("scoreboard.json")

	irc.HandleFunc("307", func(c *client.Conn, l *client.Line) {
		if l.Args[0] == nick {
			addAdmin(l.Args[1])
			sendMessage(l.Args[1], "you know what you doing")
		}
		//log.Println("\n\n---\ngot auth !!\n")
		//log.Printf("%#v %#v\n", c, l)
	})
	//irc.HandleFunc("318", func(c *client.Conn, l *client.Line) {
	//log.Println("\n\n---\ngot end of whois\n\n")
	//log.Printf("%#v %#v\n", c, l)
	//})
	irc.HandleFunc(client.PRIVMSG, func(c *client.Conn, l *client.Line) {
		//log.Printf("%#v\n", l)
		who, msg := l.Args[0], l.Args[1]
		if who == nick {
			who = l.Nick
		}
		parseMessage(who, msg, l.Nick)
		if cmdRegexp.MatchString(msg) {
			matches := cmdRegexp.FindAllStringSubmatch(msg, -1)
			if len(matches) == 0 {
				return
			}
			for _, m := range matches {
				new := ""
				if e, ok := emoji[m[1]]; ok {
					new = e
				} else if o, ok := other[m[1]]; ok {
					new = o[rand.Intn(len(o))]
				} else if j, ok := jmote[m[1]]; ok {
					new = j[rand.Intn(len(j))]
				} else {
					return
				}
				msg = strings.Replace(msg, m[0], new, 1)
			}
			irc.Privmsg(who, msg)
			return
		}
	})

	if err := irc.ConnectTo(host); err != nil {
		log.Fatalln("Connection error:", err)
	}

	<-ded
	log.Println("disconnected.")
	os.Exit(1)

}

func checkErr(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

func fileExists(filename string) bool {
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		return false
	}
	checkErr(f.Close())
	checkErr(err)
	return true
}

func fileGetContents(filename string) []byte {
	contents := new(bytes.Buffer)
	f, err := os.Open(filename)
	checkErr(err)
	_, err = io.Copy(contents, f)
	f.Close()
	if err != io.EOF {
		checkErr(err)
	}
	return contents.Bytes()
}

func filePutContents(filename string, contents []byte) {
	f, err := os.Create(filename)
	checkErr(err)
	_, err = f.Write(contents)
	checkErr(err)
	checkErr(f.Close())
}
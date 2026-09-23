package utils

import "github.com/ohaiibuzzle/BuzzUtils3/src/command"

func Ping(c *command.Ctx) {
	c.Reply("Pong!\nLatency: " + c.Session.HeartbeatLatency().String())
}

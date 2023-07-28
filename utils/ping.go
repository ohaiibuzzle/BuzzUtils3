package utils

import "github.com/bwmarrin/discordgo"

func Ping(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	ctx.ChannelMessageSend(msg.ChannelID, "Pong!")
	// Send latency
	ctx.ChannelMessageSend(msg.ChannelID, "Latency: "+ctx.HeartbeatLatency().String())
}

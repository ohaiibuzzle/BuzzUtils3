package utils

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

func Sudo(c *command.Ctx) {
	c.DeleteInvocation()
	c.Session.ChannelTyping(c.ChannelID)
	c.Session.ChannelMessageSendComplex(c.ChannelID, &discordgo.MessageSend{
		Content:         c.String("text"),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
}

func LeaveServer(c *command.Ctx) {
	c.ReplyPrivate("Bye! 👋")
	c.Session.GuildLeave(c.GuildID)
}

func Help(c *command.Ctx) {
	prefix := config.GetConfig().BotPrefix
	var sb strings.Builder
	for _, cmd := range command.All() {
		if cmd.OwnerOnly {
			continue
		}
		sb.WriteString("`" + prefix + cmd.Name + "`")
		if len(cmd.Aliases) > 0 {
			sb.WriteString(" (" + strings.Join(cmd.Aliases, ", ") + ")")
		}
		sb.WriteString(" - " + cmd.Description + "\n")
	}

	c.ReplyEmbed(&discordgo.MessageEmbed{
		Title:       "Commands",
		Description: sb.String(),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Every command also works as a slash command. Right click a message → Apps for more!",
		},
	})
}

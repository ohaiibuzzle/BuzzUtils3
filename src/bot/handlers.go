package bot

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/birthdays"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
	"github.com/ohaiibuzzle/BuzzUtils3/src/database"
	messagesutils "github.com/ohaiibuzzle/BuzzUtils3/src/messagesUtils"
	"github.com/ohaiibuzzle/BuzzUtils3/src/welcome"

	// These packages register their commands on import
	_ "github.com/ohaiibuzzle/BuzzUtils3/src/fun"
	_ "github.com/ohaiibuzzle/BuzzUtils3/src/getimages"
	_ "github.com/ohaiibuzzle/BuzzUtils3/src/imageClassifier"
	_ "github.com/ohaiibuzzle/BuzzUtils3/src/nsfwrole"
	_ "github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
	_ "github.com/ohaiibuzzle/BuzzUtils3/src/utils"
)

func RegisterHandlers(s *discordgo.Session) {
	s.AddHandler(OnReadyHandler)
	s.AddHandler(OnMessageHandler)
	s.AddHandler(command.HandleInteraction)
	s.AddHandler(welcome.OnMemberJoin)
	s.AddHandler(OnMemberRemove)
	s.AddHandler(OnGuildDelete)
}

func OnReadyHandler(s *discordgo.Session, event *discordgo.Ready) {
	log.Default().Println("Logged in as " + event.User.Username)
	s.UpdateGameStatus(0, "in Buzzle's Box. Available on GitHub")

	if err := command.Sync(s); err != nil {
		log.Default().Println("Error registering slash commands: " + err.Error())
	}

	birthdays.StartBirthdayCoroutine(s)
}

func OnMessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Prevent a loop of doom, and a multi-bot mess
	if m.Author == nil || m.Author.ID == s.State.User.ID || m.Author.Bot {
		return
	}

	if messagesutils.HandleSaveShortcut(s, m) {
		return
	}

	command.HandleMessage(s, m, config.GetConfig().BotPrefix)
}

// OnMemberRemove forgets a member's per-guild data when they leave.
func OnMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if err := (&birthdays.Birthday{}).DeleteBirthday(m.GuildID, m.User.ID); err != nil {
		log.Default().Println("Error removing birthday: " + err.Error())
	}
	database.ForgetMember(m.GuildID, m.User.ID)
}

// OnGuildDelete forgets a guild's data when the bot is removed from it.
func OnGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	// Unavailable means a Discord outage, not a removal
	if g.Unavailable {
		return
	}
	if err := birthdays.ForgetGuild(g.ID); err != nil {
		log.Default().Println("Error removing guild birthdays: " + err.Error())
	}
	database.ForgetGuild(g.ID)
}

package bot

import (
	"log"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
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

func RegisterHandlers(client *bot.Client) {
	client.AddEventListeners(
		bot.NewListenerFunc(OnReadyHandler),
		bot.NewListenerFunc(OnMessageHandler),
		bot.NewListenerFunc(command.HandleCommand),
		bot.NewListenerFunc(command.HandleComponent),
		bot.NewListenerFunc(welcome.OnMemberJoin),
		bot.NewListenerFunc(OnMemberRemove),
		bot.NewListenerFunc(OnGuildLeave),
	)
}

func OnReadyHandler(e *events.Ready) {
	log.Default().Println("Logged in as " + e.User.Username)

	if err := command.Sync(e.Client()); err != nil {
		log.Default().Println("Error registering slash commands: " + err.Error())
	}

	birthdays.StartBirthdayCoroutine(e.Client())
}

func OnMessageHandler(e *events.MessageCreate) {
	// Prevent a loop of doom, and a multi-bot mess
	if e.Message.Author.ID == e.Client().ID() || e.Message.Author.Bot {
		return
	}

	if messagesutils.HandleSaveShortcut(e) {
		return
	}

	command.HandleMessage(e, config.GetConfig().BotPrefix)
}

// OnMemberRemove forgets a member's per-guild data when they leave.
func OnMemberRemove(e *events.GuildMemberLeave) {
	if err := birthdays.DeleteBirthday(e.GuildID, e.User.ID); err != nil {
		log.Default().Println("Error removing birthday: " + err.Error())
	}
	database.ForgetMember(e.GuildID, e.User.ID)
}

// OnGuildLeave forgets a guild's data when the bot is removed from it. disgo reports
// outages separately (GuildUnavailable), so this only fires on a real removal.
func OnGuildLeave(e *events.GuildLeave) {
	if err := birthdays.ForgetGuild(e.GuildID); err != nil {
		log.Default().Println("Error removing guild birthdays: " + err.Error())
	}
	database.ForgetGuild(e.GuildID)
}

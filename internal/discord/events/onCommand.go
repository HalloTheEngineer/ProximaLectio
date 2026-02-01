package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"proximaLectio/internal/database/models/untis"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const (
	Today = TargetPeriod(iota)
	Tomorrow
	Week

	MsgSchoolNotFound = "Could not find the school you provided."
	MsgSchoolFailed   = "Could not find school. Are you logged in?"
	MsgNotLoggedIn    = "You are not logged in."
)

var (
	t = true
	f = false
)

type TargetPeriod int

func (h *Handler) CommandListener(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	switch data.CommandName() {
	case "login":
		h.handleLogin(e)
	case "logout":
		h.handleLogout(e)
	case "school":
		h.handleSchool(e)

	case "pull":
		h.handlePull(e)
	case "today":
		h.handleSchedule(e, Today)
	case "tomorrow":
		h.handleSchedule(e, Tomorrow)
	case "week":
		h.handleSchedule(e, Week)
	case "room":
		h.handleRoom(e)

	case "setup":
		h.handleSetup(e)
	case "notifications":
		h.handleNotification(e)

	case "absences":
		h.handleAbsences(e)

	case "theme":
		h.handleTheme(e)
	}
}

func (h *Handler) handleLogin(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// check existence of user
	if h.DB.Untis.UserExists(ctx, e.User().ID.String()) {
		_ = e.CreateMessage(getErrorEmbed("You are logged in, log out first!", nil))
		return
	}

	// validate params
	params, param := parseParams(&data, "institution", "username", "password")
	if param != nil {
		_ = e.CreateMessage(getErrorEmbed(fmt.Sprintf("Please provide a valid %s!", *param), nil))
		return
	}

	inst := params[0].(string)

	// find school
	var school *untis.School
	var err error

	var query, tenantID string
	if _, err := strconv.Atoi(inst); err == nil {
		tenantID = inst
	} else {
		query = inst
	}

	schools, err := h.DB.Untis.SearchSchools(ctx, query, tenantID)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed(MsgSchoolNotFound, err))
		return
	}
	if len(schools) == 0 {
		_ = e.CreateMessage(getErrorEmbed(MsgSchoolNotFound, nil))
		return
	}
	school = &schools[0]

	user, err := h.DB.Untis.LoginUser(ctx, school, params[1].(string), params[2].(string), e.User().ID.String(), e.User().Username)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed(err.Error(), nil))
		return
	}

	n := time.Now()
	err = h.DB.Untis.SyncUserTimetable(ctx, e.User().ID.String(), FloorToDay(n), EndOfDay(n.AddDate(0, 0, 7)))
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("Logged in, but failed to pull schedule", err))
		return
	}

	_ = e.CreateMessage(getSuccessEmbed("You are logged in now!", discord.EmbedField{
		Name:   "User",
		Value:  fmt.Sprintf("```\nName: %s\nSchool: %s\n```", user.DisplayName, school.DisplayName),
		Inline: nil,
	}))
}

func (h *Handler) handleLogout(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	id := e.User().ID.String()

	if !h.DB.Untis.LogoutUser(ctx, id) {
		_ = e.CreateMessage(getErrorEmbed("An error occurred while logging out.\nPerhaps, you weren't logged in?", nil))
		return
	}

	_ = e.CreateMessage(getSuccessEmbed("Logged out successfully!"))
}

func (h *Handler) handleSchool(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var err error
	var user *untis.User
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	school, err := h.DB.Untis.GetSchool(ctx, strconv.FormatInt(user.UntisSchoolID, 10))
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("Failed to fetch school", err))
		return
	}

	_ = e.CreateMessage(
		getSuccessEmbed("This is your currently connected school!",
			discord.EmbedField{
				Name:   "Name",
				Value:  codeBloc(school.DisplayName),
				Inline: &t,
			},
			discord.EmbedField{
				Name:   "Address",
				Value:  codeBloc(school.Address),
				Inline: &t,
			},
			discord.EmbedField{
				Name:   "WebUntis Url",
				Value:  fmt.Sprintf("https://%s/Webuntis", school.Server),
				Inline: &f,
			},
		),
	)
}

func (h *Handler) handlePull(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	var err error
	if _, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	n := time.Now()

	err = h.DB.Untis.SyncUserTimetable(ctx, e.User().ID.String(), FloorToDay(n), EndOfDay(n.AddDate(0, 0, 7)))
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("Failed to pull schedule", err))
		return
	}

	_ = e.CreateMessage(getSuccessEmbed("Pulled schedule successfully!"))
}

func (h *Handler) handleSchedule(e *events.ApplicationCommandInteractionCreate, period TargetPeriod) {
	data := e.SlashCommandInteractionData()

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	var periodStr string
	var start, end time.Time
	var dayCount = 1

	now := time.Now()

	switch period {
	case Today:
		periodStr = "today"
		start, end = now, now
	case Tomorrow:
		periodStr = "tomorrow"
		t := now.AddDate(0, 0, 1)
		start, end = t, t
	case Week:
		periodStr = "the week"

		if data.String("target") == "next" {
			now = now.AddDate(0, 0, 7)
		}

		start, end = getWeekRange(now)
		dayCount = 5
	default:
		slog.Error("Unhandled period type", "period", period)
		return
	}

	timetable, err := h.DB.Untis.GetTimetable(ctx, e.User().ID.String(), start, end)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("An error occurred while fetching timetable", err))
		return
	}

	if len(timetable.Days) == 0 || (start.Equal(end) && (start.Weekday() == time.Saturday || start.Weekday() == time.Sunday)) {
		_ = e.CreateMessage(getWarnEmbed(fmt.Sprintf("There is no schedule for %s.", periodStr)))
		return
	}

	_ = e.DeferCreateMessage(true)

	image, err := h.DB.Untis.GenerateScheduleImage(timetable, dayCount, user.ThemeID)
	if err != nil {
		_ = updateInteractionResp(h.Bot, e.Token(), getErrorUpdateEmbed("An error occurred while generating schedule", err))
		return
	}

	_ = updateInteractionResp(h.Bot, e.Token(), getSuccessFileUpdateEmbed("schedule.png", fmt.Sprintf("The schedule of %s", periodStr), image))
}

func (h *Handler) handleRoom(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var err error
	var user *untis.User
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	inp := data.String("subject")

	result, err := h.DB.Untis.GetNextRoomForSubject(ctx, user.ID, inp)

	if err == nil && result == nil {
		_ = h.DB.Untis.SyncUserTimetable(ctx, user.ID, time.Now(), time.Now().AddDate(0, 0, 7))
		result, err = h.DB.Untis.GetNextRoomForSubject(ctx, user.ID, inp)
	}

	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("Failed to query the schedule.", err))
		return
	}

	if result == nil {
		queryText := "upcoming lessons"
		if inp != "" {
			queryText = fmt.Sprintf("upcoming lessons for **%s**", inp)
		}
		_ = e.CreateMessage(getWarnEmbed(fmt.Sprintf("I couldn't find any %s even after syncing.", queryText)))
		return
	}

	var timeLabel string
	if result.IsNow {
		timeLabel = "is happening **now**"
	} else if result.IsToday {
		timeLabel = fmt.Sprintf("starts **today at %s**", result.StartTime.Format("15:04"))
	} else {
		timeLabel = fmt.Sprintf("starts on **%s at %s**", result.StartTime.Format("Monday"), result.StartTime.Format("15:04"))
	}

	msg := fmt.Sprintf("Your lesson **%s** %s in room **%s** (Teacher: %s).",
		result.Subject,
		timeLabel,
		result.Room,
		result.Teacher,
	)

	_ = e.CreateMessage(getSuccessEmbed(msg))
}

func (h *Handler) handleSetup(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !e.Member().Permissions.Has(discord.PermissionManageChannels) {
		_ = e.CreateMessage(getWarnEmbed("You need the `Manage Channels` permission to use this command."))
		return
	}

	guild, b := e.Guild()
	if !b {
		_ = e.CreateMessage(getWarnEmbed("This command can only be used within a server."))
		return
	}

	err := h.DB.RegisterGuild(ctx, guild.ID.String(), guild.Name)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("An error occurred while registering guild", err))
		return
	}

	subGroup := data.SubCommandGroupName // "notifications"
	subCommand := data.SubCommandName

	if *subGroup == "notifications" {
		h.handleSetupNotifications(ctx, e, *subCommand, data)
	}
}

func (h *Handler) handleSetupNotifications(ctx context.Context, e *events.ApplicationCommandInteractionCreate, action string, data discord.SlashCommandInteractionData) {
	channelID := e.Channel().ID().String()
	if channel, ok := data.OptChannel("channel"); ok {
		channelID = channel.ID.String()
	}

	guildID := e.GuildID().String()

	switch action {
	case "allow":
		err := h.DB.Untis.AllowChannel(ctx, guildID, channelID)
		if err != nil {
			_ = e.CreateMessage(getErrorEmbed("Failed to allow channel", err))
			return
		}

		_ = e.CreateMessage(getSuccessEmbed(
			fmt.Sprintf("Users are now permitted to set their notification target to <#%s>.", channelID),
		))

	case "revoke":
		err := h.DB.Untis.RevokeChannel(ctx, guildID, channelID)
		if err != nil {
			_ = e.CreateMessage(getErrorEmbed("Failed to revoke channel", err))
			return
		}

		_ = e.CreateMessage(getSuccessEmbed(
			fmt.Sprintf("Authorization Revoked.\nChannel <#%s> has been removed from the allow-list.", channelID),
		))
	}
}

func (h *Handler) handleNotification(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	subcommand := data.SubCommandName

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	var user *untis.User
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	switch *subcommand {
	case "status":
		h.handleNotificationStatus(e, user)
	case "set":
		h.handleNotificationSet(e, user, data)
	}
}

func (h *Handler) handleNotificationStatus(e *events.ApplicationCommandInteractionCreate, user *untis.User) {
	statusEmoji := "❌ Disabled"
	if user.NotificationsEnabled {
		statusEmoji = "✅ Enabled"
	}

	target := user.NotificationTarget
	if target == "" {
		target = "DM (Default)"
	}

	address := user.NotificationAddress
	if address == "" {
		address = "N/A"
	} else if target == "WEBHOOK" {
		address = "`Redacted for security`"
	}

	_ = e.CreateMessage(
		getSuccessEmbed("Use `/notifications set` to change the configuration.",
			discord.EmbedField{
				Name:   "Status",
				Value:  codeBloc(statusEmoji),
				Inline: &t,
			},
			discord.EmbedField{
				Name:   "Target",
				Value:  codeBloc(target),
				Inline: &t,
			},
			discord.EmbedField{
				Name:   "Address",
				Value:  codeBloc(address),
				Inline: &f,
			},
		),
	)
}

func (h *Handler) handleNotificationSet(e *events.ApplicationCommandInteractionCreate, user *untis.User, data discord.SlashCommandInteractionData) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	enabled, hasEnabled := data.OptBool("enabled")
	if !hasEnabled {
		enabled = user.NotificationsEnabled
	}

	target, hasTarget := data.OptString("target")
	if !hasTarget {
		target = user.NotificationTarget
	}

	address, hasAddress := data.OptString("address")
	if !hasAddress {
		address = user.NotificationAddress
	}

	if !hasEnabled && !hasTarget && !hasAddress {
		_ = e.CreateMessage(getWarnEmbed("You need to provide at least one argument."))
		return
	}

	if target != "WEBHOOK" && hasAddress {
		_ = e.CreateMessage(getWarnEmbed("Change the target to set a webhook URL."))
		return
	}

	if target == "CHANNEL" {
		address = e.Channel().ID().String()

		isAllowed, err := h.DB.Untis.IsChannelAllowed(ctx, e.GuildID().String(), address)
		if err != nil {
			_ = e.CreateMessage(getErrorEmbed("Error while checking for clearance", err))
			return
		}
		if !isAllowed {
			_ = e.CreateMessage(getWarnEmbed("You cant use this channel until the admin has allowed it.\nCommand: `/setup notifications allow`"))
			return
		}
	}

	if target == "WEBHOOK" && !strings.HasPrefix(address, "https://discord.com/api/webhooks/") {
		_ = e.CreateMessage(getErrorEmbed("Invalid Webhook URL", fmt.Errorf("webhook URLs must start with `https://discord.com/api/webhooks/`")))
		return
	}

	err := h.DB.Untis.SetNotificationConfig(ctx, user.ID, enabled, target, address)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("Failed to update settings", err))
		return
	}

	statusText := "disabled"
	if enabled {
		statusText = "enabled"
	}

	if address == "" {
		address = "N/A"
	}

	if enabled {
		h.sendNotification(
			ctx,
			user.ID,
			untis.NotificationTarget{Type: target, Address: address},
			discord.
				NewEmbedBuilder().
				SetTimestamp(time.Now()).
				SetColor(9036596).
				SetTitle("Update <a:alert:1467490337839648818>").
				SetDescription(fmt.Sprintf("<@%s>\nYou are now receiving notifications here.", user.ID)).
				Build(),
		)
	}

	err = e.CreateMessage(getSuccessEmbed(fmt.Sprintf("Notifications are now **%s**.\nTarget: `%s`\nAddress: `%s`", statusText, target, address)))
	if err != nil {
		slog.Error(err.Error())
	}
}

func (h *Handler) handleAbsences(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	filter := 0
	if f, ok := data.OptInt("filter"); ok {
		filter = f
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	_ = h.DB.Untis.SyncUserAbsences(ctx, user.ID)

	records, err := h.DB.Untis.GetUserAbsences(ctx, user.ID, filter)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("Failed to retrieve absences from database.", err))
		return
	}

	if len(records) == 0 {
		_ = e.CreateMessage(getSuccessEmbed("You have no recorded absences matching this filter."))
		return
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Your Absences").
		SetColor(0x5865F2).
		SetDescription("Here are your recorded absences:")

	for i, r := range records {
		if i >= 10 {
			embed.SetFooter(fmt.Sprintf("...and %d more", len(records)-10), "")
			break
		}

		status := "Unexcused ✗"
		if r.IsExcused {
			status = "Excused ✔"
		}

		dateStr := r.StartDate.Format("02.01.2006")
		if !r.StartDate.Equal(r.EndDate) {
			dateStr = fmt.Sprintf("%s - %s", dateStr, r.EndDate.Format("02.01.2006"))
		}

		reason := r.Reason
		if reason == "" {
			reason = "No reason provided"
		}

		embed.AddField(fmt.Sprintf("%s (%s)", dateStr, status), reason, false)
	}

	_ = e.CreateMessage(discord.NewMessageCreateBuilder().SetEphemeral(true).SetEmbeds(embed.Build()).Build())
}

func (h *Handler) handleTheme(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	params, param := parseParams(&data, "theme")
	if param != nil {
		_ = e.CreateMessage(getSuccessEmbed(fmt.Sprintf("Your current theme is: `%s`", strings.ToUpper(user.ThemeID))))
		return
	}

	themeStr := strings.ToLower(params[0].(string))

	_, err = h.DB.Untis.GetTheme(themeStr)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("Could not find the specified theme.", nil))
		return
	}

	err = h.DB.Untis.SetTheme(ctx, e.User().ID.String(), themeStr)
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed("An error occurred while setting the theme", err))
	}

	_ = e.CreateMessage(getSuccessEmbed(fmt.Sprintf("Theme updated successfully to `%s`!", strings.ToUpper(themeStr))))
}

func (h *Handler) ensureLogin(ctx context.Context, e *events.ApplicationCommandInteractionCreate) (*untis.User, error) {
	user, err := h.DB.Untis.GetUser(ctx, e.User().ID.String())
	if err != nil {
		_ = e.CreateMessage(getErrorEmbed(MsgNotLoggedIn, err))
		return nil, err
	}

	if user == nil {
		_ = e.CreateMessage(getErrorEmbed(MsgNotLoggedIn, err))
		return nil, errors.New("user not found")
	}
	return user, nil
}

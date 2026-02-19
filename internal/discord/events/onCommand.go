package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"proximaLectio/internal/constants"
	"proximaLectio/internal/database/models/untis"
	"proximaLectio/internal/utils"
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
	case "exams":
		h.handleExams(e)
	case "homework":
		h.handleHomework(e)
	case "stats":
		h.handleStats(e)
	case "common":
		h.handleCommon(e)
	case "excuse":
		h.handleExcuse(e)

	case "theme":
		h.handleTheme(e)
	}
}

func (h *Handler) handleLogin(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	_ = e.DeferCreateMessage(true)

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutLong)
	defer cancel()

	userID := e.User().ID.String()
	b := *h.Bot
	if b == nil {
		return
	}

	if h.DB.Untis.UserExists(ctx, userID) {
		_, _ = b.Rest().UpdateInteractionResponse(e.ApplicationID(), e.Token(), utils.GetWarnUpdateEmbed("You are already logged in. Please use `/logout` first if you want to switch accounts."))
		return
	}

	institution, _ := data.OptString("institution")
	username, _ := data.OptString("username")
	password, _ := data.OptString("password")

	var school *untis.School
	var query, tenantID string
	if _, err := strconv.Atoi(institution); err == nil {
		tenantID = institution
	} else {
		query = institution
	}

	schools, err := h.DB.Untis.SearchSchools(ctx, query, tenantID)
	if err != nil || len(schools) == 0 {
		_, _ = b.Rest().UpdateInteractionResponse(e.ApplicationID(), e.Token(), utils.GetErrorUpdateEmbed("Could not find the specified school. Please use the search suggestions.", err))
		return
	}
	school = &schools[0]

	user, err := h.DB.Untis.LoginUser(ctx, school, username, password, userID, e.User().Username)
	if err != nil {
		_, _ = b.Rest().UpdateInteractionResponse(e.ApplicationID(), e.Token(), utils.GetErrorUpdateEmbed("Login failed. Please check your username and password.", err))
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(userID, e.GuildID().String())
	}

	now := time.Now()
	err = h.DB.Untis.SyncUserTimetable(ctx, userID, utils.FloorToDay(now), utils.EndOfDay(now.AddDate(0, 0, 7)))

	successEmbed := discord.NewEmbedBuilder().
		SetTitle("🚀 Successfully Logged In").
		SetColor(constants.ColorSuccess).
		SetDescription(fmt.Sprintf("Welcome, **%s**! Your account is now linked to [**%s**](https://%s).", user.DisplayName, school.DisplayName, school.Server))

	if err != nil {
		successEmbed.SetDescription(successEmbed.Description + "\n\n⚠️ *Login was successful, but I couldn't fetch your schedule yet. It will sync automatically in the background.*")
	}

	successEmbed.AddField("School", fmt.Sprintf("`%s`", school.DisplayName), true)
	successEmbed.AddField("Status", "✅ Initial Sync Complete", true)

	_, _ = b.Rest().UpdateInteractionResponse(e.ApplicationID(), e.Token(), discord.NewMessageUpdateBuilder().
		SetEmbeds(successEmbed.Build()).
		Build(),
	)
}

func (h *Handler) handleLogout(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutShort)
	defer cancel()

	id := e.User().ID.String()

	if !h.DB.Untis.LogoutUser(ctx, id) {
		_ = e.CreateMessage(utils.GetErrorEmbed("An error occurred while logging out.\nPerhaps, you weren't logged in?", nil))
		return
	}

	_ = e.CreateMessage(utils.GetSuccessEmbed("Logged out successfully!"))
}

func (h *Handler) handleSchool(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutMedium)
	defer cancel()

	var err error
	var user *untis.User
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	school, err := h.DB.Untis.GetSchool(ctx, strconv.FormatInt(user.UntisSchoolID, 10))
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to fetch school", err))
		return
	}

	_ = e.CreateMessage(
		utils.GetSuccessEmbed("This is your currently connected school!",
			discord.EmbedField{
				Name:   "Name",
				Value:  utils.CodeBloc(school.DisplayName),
				Inline: &t,
			},
			discord.EmbedField{
				Name:   "Address",
				Value:  utils.CodeBloc(school.Address),
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
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutMedium)
	defer cancel()

	var err error
	if _, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(e.User().ID.String(), e.GuildID().String())
	}

	h.DB.Untis.Sync(ctx, e.User().ID.String())

	_ = e.CreateMessage(utils.GetSuccessEmbed("Data has been pulled!"))
}

func (h *Handler) handleSchedule(e *events.ApplicationCommandInteractionCreate, period TargetPeriod) {
	data := e.SlashCommandInteractionData()

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutRender)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(user.ID, e.GuildID().String())
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

		start, end = utils.GetWeekRange(now)
		dayCount = 5
	default:
		slog.Error("Unhandled period type", "period", period)
		return
	}

	timetable, err := h.DB.Untis.GetTimetable(ctx, e.User().ID.String(), start, end)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("An error occurred while fetching timetable", err))
		return
	}

	if len(timetable.Days) == 0 || (start.Equal(end) && (start.Weekday() == time.Saturday || start.Weekday() == time.Sunday)) {
		_ = e.CreateMessage(utils.GetWarnEmbed(fmt.Sprintf("There is no schedule for %s.", periodStr)))
		return
	}

	_ = e.DeferCreateMessage(false)

	image, err := h.DB.Untis.GenerateScheduleImage(timetable, dayCount, user.ThemeID)
	if err != nil {
		_ = updateInteractionResp(h.Bot, e.Token(), utils.GetErrorUpdateEmbed("An error occurred while generating schedule", err))
		return
	}

	_ = updateInteractionResp(h.Bot, e.Token(), utils.GetSuccessFileUpdateEmbed("schedule.png", fmt.Sprintf("The schedule of %s", periodStr), image))
}

func (h *Handler) handleRoom(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutLong)
	defer cancel()

	var err error
	var user *untis.User
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(e.User().ID.String(), e.GuildID().String())
	}

	inp := data.String("subject")

	result, err := h.DB.Untis.GetNextRoomForSubject(ctx, user.ID, inp)

	if err == nil && result == nil {
		_ = h.DB.Untis.SyncUserTimetable(ctx, user.ID, time.Now(), time.Now().AddDate(0, 0, 7))
		result, err = h.DB.Untis.GetNextRoomForSubject(ctx, user.ID, inp)
	}

	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to query the schedule.", err))
		return
	}

	if result == nil {
		queryText := "upcoming lessons"
		if inp != "" {
			queryText = fmt.Sprintf("upcoming lessons for **%s**", inp)
		}
		_ = e.CreateMessage(utils.GetWarnEmbed(fmt.Sprintf("I couldn't find any %s even after syncing.", queryText)))
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

	_ = e.CreateMessage(discord.NewMessageCreateBuilder().SetEmbeds(
		discord.NewEmbedBuilder().
			SetTimestamp(time.Now()).
			SetColor(constants.ColorPrimary).
			SetTitle("Room Found").
			SetDescription(msg).
			Build(),
	).Build())
}

func (h *Handler) handleSetup(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutShort)
	defer cancel()

	if !e.Member().Permissions.Has(discord.PermissionManageChannels) {
		_ = e.CreateMessage(utils.GetWarnEmbed("You need the `Manage Channels` permission to use this command."))
		return
	}

	guild, b := e.Guild()
	if !b {
		_ = e.CreateMessage(utils.GetWarnEmbed("This command can only be used within a server."))
		return
	}

	err := h.DB.RegisterGuild(ctx, guild.ID.String(), guild.Name)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("An error occurred while registering guild", err))
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
			_ = e.CreateMessage(utils.GetErrorEmbed("Failed to allow channel", err))
			return
		}

		_ = e.CreateMessage(utils.GetSuccessEmbed(
			fmt.Sprintf("Users are now permitted to set their notification target to <#%s>.", channelID),
		))

	case "revoke":
		err := h.DB.Untis.RevokeChannel(ctx, guildID, channelID)
		if err != nil {
			_ = e.CreateMessage(utils.GetErrorEmbed("Failed to revoke channel", err))
			return
		}

		_ = e.CreateMessage(utils.GetSuccessEmbed(
			fmt.Sprintf("Authorization Revoked.\nChannel <#%s> has been removed from the allow-list.", channelID),
		))
	}
}

func (h *Handler) handleNotification(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	subcommand := data.SubCommandName

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutShort)
	defer cancel()

	var err error
	var user *untis.User
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(e.User().ID.String(), e.GuildID().String())
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
		utils.GetSuccessEmbed("Use `/notifications set` to change the configuration.\nUnless you've enabled notifications, no stats about your schedule are collected.",
			discord.EmbedField{
				Name:   "Status",
				Value:  utils.CodeBloc(statusEmoji),
				Inline: &t,
			},
			discord.EmbedField{
				Name:   "Target",
				Value:  utils.CodeBloc(target),
				Inline: &t,
			},
			discord.EmbedField{
				Name:   "Address",
				Value:  utils.CodeBloc(address),
				Inline: &f,
			},
		),
	)
}

func (h *Handler) handleNotificationSet(e *events.ApplicationCommandInteractionCreate, user *untis.User, data discord.SlashCommandInteractionData) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutShort)
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
		_ = e.CreateMessage(utils.GetWarnEmbed("You need to provide at least one argument."))
		return
	}

	if target != "WEBHOOK" && hasAddress {
		_ = e.CreateMessage(utils.GetWarnEmbed("Change the target to set a webhook URL."))
		return
	}

	if target == "CHANNEL" {
		address = e.Channel().ID().String()

		isAllowed, err := h.DB.Untis.IsChannelAllowed(ctx, e.GuildID().String(), address)
		if err != nil {
			_ = e.CreateMessage(utils.GetErrorEmbed("Error while checking for clearance", err))
			return
		}
		if !isAllowed {
			_ = e.CreateMessage(utils.GetWarnEmbed("You cant use this channel until the admin has allowed it.\nCommand: `/setup notifications allow`"))
			return
		}
	}

	if target == "WEBHOOK" && !strings.HasPrefix(address, "https://discord.com/api/webhooks/") {
		_ = e.CreateMessage(utils.GetErrorEmbed("Invalid Webhook URL", fmt.Errorf("webhook URLs must start with `https://discord.com/api/webhooks/`")))
		return
	}

	err := h.DB.Untis.SetNotificationConfig(ctx, user.ID, enabled, target, address)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to update settings", err))
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
				SetColor(constants.ColorPrimary).
				SetTitle("Update <a:alert:1467490337839648818>").
				SetDescription(fmt.Sprintf("<@%s>\nYou are now receiving notifications here.", user.ID)).
				Build(),
		)
	}

	err = e.CreateMessage(utils.GetSuccessEmbed(fmt.Sprintf("Notifications are now **%s**.\nTarget: `%s`\nAddress: `%s`", statusText, target, address)))
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

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutMedium)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(e.User().ID.String(), e.GuildID().String())
	}

	_ = h.DB.Untis.SyncUserAbsences(ctx, user.ID)

	records, err := h.DB.Untis.GetUserAbsences(ctx, user.ID, filter)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to retrieve absences from database.", err))
		return
	}

	if len(records) == 0 {
		_ = e.CreateMessage(utils.GetSuccessEmbed("You have no recorded absences matching this filter."))
		return
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Your Absences").
		SetColor(constants.ColorPrimary).
		SetDescription("Here are your recorded absences of the current school-year:")

	for i, r := range records {
		if i >= 10 {
			embed.SetFooter(fmt.Sprintf("...and %d more", len(records)-10), "")
			break
		}

		str := " ✗"
		if r.IsExcused {
			str = " ✔"
		}

		dateStr := r.StartDate.Format("02.01.2006")
		if !r.StartDate.Equal(r.EndDate) {
			dateStr = fmt.Sprintf("%s - %s", dateStr, r.EndDate.Format("02.01.2006"))
		}

		reason := r.Reason
		if reason == "" {
			reason = "No reason provided"
		}

		embed.AddField(fmt.Sprintf("%s (%s)", dateStr, r.Status+str), reason, false)
	}

	_ = e.CreateMessage(discord.NewMessageCreateBuilder().SetEphemeral(true).SetEmbeds(embed.Build()).Build())
}

func (h *Handler) handleExams(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutLong)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(e.User().ID.String(), e.GuildID().String())
	}

	_ = h.DB.Untis.SyncUserExams(ctx, user.ID)

	exams, err := h.DB.Untis.GetUpcomingExams(ctx, user.ID)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to retrieve exams.", err))
		return
	}

	if len(exams) == 0 {
		_ = e.CreateMessage(utils.GetWarnEmbed("You have no upcoming exams scheduled."))
		return
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("📝 Upcoming Exams").
		SetColor(constants.ColorInfo).
		SetDescription("Here are your upcoming tests and exams:")

	for _, ex := range exams {
		title := fmt.Sprintf("**%s** (%s)", ex.Subject, ex.Date.Format("02.01.2006"))
		details := fmt.Sprintf("⏰ %s - %s\n🏷️ %s", ex.StartTime, ex.EndTime, ex.Name)
		if ex.Name == "" {
			details = fmt.Sprintf("⏰ %s - %s", ex.StartTime, ex.EndTime)
		}
		embed.AddField(title, details, false)
	}

	_ = e.CreateMessage(discord.NewMessageCreateBuilder().SetEmbeds(embed.Build()).Build())
}

func (h *Handler) handleHomework(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	filter := 0
	if f, ok := data.OptInt("filter"); ok {
		filter = f
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutMedium)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	if e.GuildID() != nil {
		h.safeSyncGuild(e.User().ID.String(), e.GuildID().String())
	}

	_ = h.DB.Untis.SyncUserHomeworks(ctx, user.ID)

	homeworks, err := h.DB.Untis.GetUserHomeworks(ctx, user.ID, filter)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to retrieve homework from database.", err))
		return
	}

	if len(homeworks) == 0 {
		_ = e.CreateMessage(utils.GetSuccessEmbed("You have no homework assignments matching this filter."))
		return
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("📚 Your Homework").
		SetColor(constants.ColorInfo).
		SetDescription("Here are your homework assignments:")

	for i, hw := range homeworks {
		if i >= 10 {
			embed.SetFooter(fmt.Sprintf("...and %d more", len(homeworks)-10), "")
			break
		}

		status := "⏳ Pending"
		if hw.Completed {
			status = "✅ Completed"
		}

		dueStr := hw.DueDate.Format("02.01.2006")
		subject := hw.Subject
		if subject == "" {
			subject = "General"
		}

		text := hw.Text
		if len(text) > 100 {
			text = text[:97] + "..."
		}

		embed.AddField(fmt.Sprintf("%s (%s) - %s", subject, dueStr, status), text, false)
	}

	_ = e.CreateMessage(discord.NewMessageCreateBuilder().SetEphemeral(true).SetEmbeds(embed.Build()).Build())
}

func (h *Handler) handleStats(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutMedium)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	stats, err := h.DB.Untis.GetUserStats(ctx, user.ID)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to calculate your statistics.", err))
		return
	}

	cancelRate := 0.0
	if stats.TotalLessons > 0 {
		cancelRate = (float64(stats.CancelledCount) / float64(stats.TotalLessons)) * 100
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Your Insights").
		SetDescription(fmt.Sprintf("Statistics for **%s** based on synchronized data.", user.Username)).
		SetColor(constants.ColorSuccess).
		SetThumbnail(e.User().EffectiveAvatarURL())

	// Timetable Section
	embed.AddField("Timetable Overview",
		fmt.Sprintf("• Total Lessons Tracked: `%d`\n• Substitutions: `%d`\n• Cancelled: `%d` (`%.1f%%`)",
			stats.TotalLessons, stats.SubstitutionCount, stats.CancelledCount, cancelRate),
		false)

	// Logistics Section
	embed.AddField("Logistics",
		fmt.Sprintf("• Most Frequented Room: **%s**\n• Upcoming Exams: `%d`",
			stats.MostVisitedRoom, stats.UpcomingExams),
		true)

	// Absence Section
	statusEmoji := "✅"
	if stats.UnexcusedAbsences > 0 {
		statusEmoji = "⚠️"
	}
	embed.AddField(fmt.Sprintf("%s Absences", statusEmoji),
		fmt.Sprintf("• Total: `%d`\n• Unexcused: `%d`",
			stats.TotalAbsences, stats.UnexcusedAbsences),
		true)

	footer := "Keep up the good work!"
	if cancelRate > 15 {
		footer = "That's a lot of free time!"
	} else if stats.UnexcusedAbsences > 5 {
		footer = "Don't forget to hand in your excuses! <a:"
	}
	embed.SetFooter(footer, "")

	_ = e.CreateMessage(discord.NewMessageCreateBuilder().SetEmbeds(embed.Build()).Build())
}

func (h *Handler) handleCommon(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	if e.GuildID() == nil {
		_ = e.CreateMessage(utils.GetWarnEmbed("This command can only be used in a guild."))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutMedium)
	defer cancel()

	if e.GuildID() != nil {
		h.safeSyncGuild(e.User().ID.String(), e.GuildID().String())
	}

	targetTime := time.Now()
	isAtRequest := false

	if data.SubCommandName != nil && *data.SubCommandName == "at" {
		isAtRequest = true
		timeInput, _ := data.OptString("time")

		parsed, err := time.Parse("15:04", timeInput)
		if err != nil {
			_ = e.CreateMessage(utils.GetWarnEmbed("Invalid time format. Please use `HH:MM` (e.g. 13:30)."))
			return
		}

		now := time.Now()
		targetTime = time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	}

	statuses, err := h.DB.Untis.GetGuildMemberStatusesAt(ctx, e.GuildID().String(), targetTime)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Failed to retrieve server schedule data.", err))
		return
	}

	if len(statuses) == 0 {
		_ = e.CreateMessage(utils.GetWarnEmbed("No registered users found in this server."))
		return
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Common Schedule of the Guild").
		SetColor(constants.ColorInfo).
		SetTimestamp(targetTime)

	var freeUsers []string
	var busyUsers []string

	for _, s := range statuses {
		mention := fmt.Sprintf("<@%s>", s.UserID)
		if s.IsFree {
			freeUsers = append(freeUsers, mention)
		} else {
			busyUsers = append(busyUsers, fmt.Sprintf("%s: **%s** in %s (%s)", mention, s.Subject, s.Room, strings.ToLower(s.Status)))
		}
	}

	timeLabel := "Currently"
	if isAtRequest {
		timeLabel = fmt.Sprintf("At %s", targetTime.Format("15:04"))
	}

	if len(freeUsers) > 0 {
		embed.AddField(fmt.Sprintf("🟢 %s Free", timeLabel), strings.Join(freeUsers, ", "), false)
	}

	if len(busyUsers) > 0 {
		embed.AddField(fmt.Sprintf("🔴 %s in Lessons", timeLabel), strings.Join(busyUsers, "\n"), false)
	}

	_ = e.CreateMessage(discord.NewMessageCreateBuilder().SetEmbeds(embed.Build()).Build())
}

func (h *Handler) handleTheme(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutMedium)
	defer cancel()

	var user *untis.User
	var err error
	if user, err = h.ensureLogin(ctx, e); err != nil {
		return
	}

	params, param := parseParams(&data, "theme")
	if param != nil {
		_ = e.CreateMessage(utils.GetSuccessEmbed(fmt.Sprintf("Your current theme is: `%s`", strings.ToUpper(user.ThemeID))))
		return
	}

	themeStr := strings.ToLower(params[0].(string))

	_, err = h.DB.Untis.GetTheme(themeStr)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("Could not find the specified theme.", nil))
		return
	}

	err = h.DB.Untis.SetTheme(ctx, e.User().ID.String(), themeStr)
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed("An error occurred while setting the theme", err))
	}

	_ = e.CreateMessage(utils.GetSuccessEmbed(fmt.Sprintf("Theme updated successfully to `%s`!", strings.ToUpper(themeStr))))
}

func (h *Handler) handleExcuse(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()

	absenceID, ok := data.OptInt("id")
	if !ok {
		_ = e.CreateMessage(utils.GetWarnEmbed("Please provide a valid Absence ID. Start typing in the 'id' field to see your recent absences."))
		return
	}
	guardian := data.String("guardian")

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutLong)
	defer cancel()

	if _, err := h.ensureLogin(ctx, e); err != nil {
		return
	}

	_ = e.DeferCreateMessage(true)

	b := *h.Bot
	if b == nil {
		return
	}

	pdfReader, err := h.DB.Untis.GenerateExcusePDF(ctx, e.User().ID.String(), absenceID, guardian)
	if err != nil {
		_, _ = b.Rest().UpdateInteractionResponse(e.ApplicationID(), e.Token(), utils.GetErrorUpdateEmbed("Failed to generate the excuse document. Make sure you selected a valid absence.", err))
		return
	}

	fileName := fmt.Sprintf("Entschuldigung_%d.pdf", absenceID)
	attachment := discord.NewFile(fileName, "", pdfReader)

	_, err = b.Rest().UpdateInteractionResponse(e.ApplicationID(), e.Token(),
		discord.NewMessageUpdateBuilder().
			SetContent("✅ Your formal excuse letter has been generated. You can download and print it below.").
			AddFiles(attachment).
			Build(),
	)

	if err != nil {
		fmt.Printf("Error sending PDF file: %v\n", err)
	}
}

func (h *Handler) ensureLogin(ctx context.Context, e *events.ApplicationCommandInteractionCreate) (*untis.User, error) {
	user, err := h.DB.Untis.GetUser(ctx, e.User().ID.String())
	if err != nil {
		_ = e.CreateMessage(utils.GetErrorEmbed(MsgNotLoggedIn, err))
		return nil, err
	}

	if user == nil {
		_ = e.CreateMessage(utils.GetErrorEmbed(MsgNotLoggedIn, err))
		return nil, errors.New("user not found")
	}
	return user, nil
}

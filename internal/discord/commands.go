package discord

import (
	dc "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/json"
)

var (
	GlobalCommands = []dc.ApplicationCommandCreate{

		// AUTH
		dc.SlashCommandCreate{
			Name:        "login",
			Description: "Register your Untis account.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Registriere deinen Untis Account.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionString{
					Name:        "institution",
					Description: "The institution of the Untis account.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Die Institution oder Schule des Untis Accounts.",
					},
					Required:     true,
					Autocomplete: true,
				},
				dc.ApplicationCommandOptionString{
					Name: "username",
					NameLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "benutzer",
					},
					Description: "The username of the Untis account.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Der Benutzer des Untis Accounts.",
					},
					Required: true,
				},
				dc.ApplicationCommandOptionString{
					Name: "password",

					NameLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "passwort",
					},
					Description: "The password of the Untis account.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Das Password des Untis Accounts.",
					},
					Required: true,
				},
			},
		}, // LOGIN
		dc.SlashCommandCreate{
			Name:        "logout",
			Description: "Clear all stored credentials and cached data, including schedules.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Entferne deinen Account aus dem Bot.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
		}, // LOGOUT
		dc.SlashCommandCreate{
			Name:        "school",
			Description: "Displays the currently linked school name and display name.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt die aktuell verknüpfte Schule.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
		}, // SCHOOL

		// TIMETABLE & STATUS
		dc.SlashCommandCreate{
			Name:        "pull",
			Description: "Pulls the current schedule from the API, storing it in the db. This is the manual way of doing it.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Speichert den aktuellen Stundenplan in der Datenbank ab, funktioniert sonst automatisch.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
		}, // PULL
		dc.SlashCommandCreate{
			Name:        "today",
			Description: "Shows a summary of today's schedule.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt den heutigen Stundenplan.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
		}, // TODAY
		dc.SlashCommandCreate{
			Name:        "tomorrow",
			Description: "Shows a summary of tomorrow's schedule.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt den morgigen Stundenplan.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
		}, // TOMORROW
		dc.SlashCommandCreate{
			Name:        "week",
			Description: "Shows a weekly overview.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt eine Übersicht der Woche.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionString{
					Name: "target",
					NameLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "zeitraum",
					},
					Description: "Sets which week to target.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Legt die Woche fest.",
					},
					Required: false,
					Choices: []dc.ApplicationCommandOptionChoiceString{
						{
							Name: "This week",
							NameLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "Diese Woche",
							},
							Value: "current",
						},
						{
							Name: "Next week",
							NameLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "Nächste Woche",
							},
							Value: "next",
						},
					},
				},
			},
		}, // WEEK
		dc.SlashCommandCreate{
			Name:        "room",
			Description: "Shows the room a subject takes place.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt den Raum, in dem das gegebene Fach stattfindet.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionString{
					Name: "subject",
					NameLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "fach",
					},
					Description: "The subject.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Das Fach",
					},
					Autocomplete: true,
				},
			},
		}, // ROOM

		// ALERTS & NOTIFICATIONS
		dc.SlashCommandCreate{
			Name:        "setup",
			Description: "Administrative configuration for guilds.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Administrative Konfiguration für Server.",
			},
			DefaultMemberPermissions: json.NewNullablePtr(dc.PermissionManageChannels),
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionSubCommandGroup{
					Name:        "notifications",
					Description: "Configure notification channel allowances",
					Options: []dc.ApplicationCommandOptionSubCommand{
						{
							Name:        "allow",
							Description: "Allow users to send notifications to a specific channel",
							Options: []dc.ApplicationCommandOption{
								dc.ApplicationCommandOptionChannel{
									Name:        "channel",
									Description: "The channel to allow (defaults to current)",
								},
							},
						},
						{
							Name:        "revoke",
							Description: "Revoke a channel from the allowed list",
							Options: []dc.ApplicationCommandOption{
								dc.ApplicationCommandOptionChannel{
									Name:        "channel",
									Description: "The channel to revoke (defaults to current)",
								},
							},
						},
					},
				},
			},
		}, // SETUP
		dc.SlashCommandCreate{
			Name:        "notifications",
			Description: "Configures automatic background alerts for schedule changes.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Konfiguriert Benachrichtigungen für Planänderungen.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionSubCommand{
					Name: "status",
					NameLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "status",
					},
					Description: "Shows the current configuration.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Zeigt die aktuelle Konfiguration.",
					},
				},
				dc.ApplicationCommandOptionSubCommand{
					Name:        "set",
					Description: "Configures a specific setting.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Konfiguriert eine bestimmte Einstellung.",
					},
					Options: []dc.ApplicationCommandOption{
						dc.ApplicationCommandOptionBool{
							Name: "enabled",
							NameLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "aktiviert",
							},
							Description: "Toggle notifications on or off",
							DescriptionLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "Schaltet Benachrichtigungen an oder aus.",
							},
						},
						dc.ApplicationCommandOptionString{
							Name: "target",
							NameLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "ziel",
							},
							Description: "Where should the alerts be sent?",
							DescriptionLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "Wohin sollen die Benachrichtigungen geschickt werden?",
							},
							Choices: []dc.ApplicationCommandOptionChoiceString{
								{
									Name: "Direct Messages (DM)",
									NameLocalizations: map[dc.Locale]string{
										dc.LocaleGerman: "Direkt-Nachrichten (DM)",
									},
									Value: "DM",
								},
								{
									Name: "This Channel",
									NameLocalizations: map[dc.Locale]string{
										dc.LocaleGerman: "Dieser Kanal",
									},
									Value: "CHANNEL",
								},
								{
									Name:  "Webhook URL",
									Value: "WEBHOOK",
								},
							},
						},
						dc.ApplicationCommandOptionString{
							Name:        "address",
							Description: "The Webhook URL (required for Webhook target)",
						},
					},
				},
			},
		}, // NOTIFICATIONS

		// ABSENCES & INFO
		dc.SlashCommandCreate{
			Name:        "absences",
			Description: "Shows a your absences.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt deine Fehlzeiten.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionInt{
					Name:        "filter",
					Description: "Filter absences.",
					Required:    false,
					Choices: []dc.ApplicationCommandOptionChoiceInt{
						{
							Name: "all",
							NameLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "alle",
							},
							Value: 0,
						},
						{
							Name: "unexcused",
							NameLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "unentschuldigt",
							},
							Value: 1,
						},
						{
							Name: "excused",
							NameLocalizations: map[dc.Locale]string{
								dc.LocaleGerman: "entschuldigt",
							},
							Value: 2,
						},
					},
				},
			},
		}, // ABSENCES
		dc.SlashCommandCreate{
			Name:        "exams",
			Description: "Shows upcoming exams.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt anstehende Prüfungen.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
		}, // EXAMS
		dc.SlashCommandCreate{
			Name:        "stats",
			Description: "Shows your academic statistics and yearly progress.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Zeigt deine akademischen Statistiken und den Jahresfortschritt.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
		}, // STATS
		dc.SlashCommandCreate{
			Name:        "common",
			Description: "Find shared schedules among server members.",
			Contexts:    []dc.InteractionContextType{dc.InteractionContextTypeGuild},
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionSubCommand{
					Name:        "free",
					Description: "Show who is currently free.",
				},
				dc.ApplicationCommandOptionSubCommand{
					Name:        "at",
					Description: "Check everyone's status at a specific time.",
					Options: []dc.ApplicationCommandOption{
						dc.ApplicationCommandOptionString{
							Name:        "time",
							Description: "The time to check (HH:MM).",
							Required:    true,
						},
					},
				},
			},
		}, // COMMON
		dc.SlashCommandCreate{
			Name:        "excuse",
			Description: "Generates a formal PDF excuse for a specific absence.",
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionInt{
					Name:         "id",
					Description:  "Select the absence from the list",
					Required:     true,
					Autocomplete: true,
				},
			},
		}, // EXCUSE

		// CUSTOMIZATION
		dc.SlashCommandCreate{
			Name:        "theme",
			Description: "Sets the color theme of rendered schedules.",
			DescriptionLocalizations: map[dc.Locale]string{
				dc.LocaleGerman: "Setzt das Farbschema des dargestellten Stundenplans.",
			},
			Contexts: []dc.InteractionContextType{dc.InteractionContextTypeBotDM, dc.InteractionContextTypeGuild},
			Options: []dc.ApplicationCommandOption{
				dc.ApplicationCommandOptionString{
					Name:        "theme",
					Description: "The selected theme.",
					DescriptionLocalizations: map[dc.Locale]string{
						dc.LocaleGerman: "Das zu setzende Schema.",
					},
					Required:     false,
					Autocomplete: true,
				},
			},
		}, // THEME
	}
)

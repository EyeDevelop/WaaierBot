package handlers

import (
	whatsapp "github.com/Rhymen/go-whatsapp"
	"fmt"
	"os"
	"time"
	"strings"
)

// WaaierHandler is a struct that handles WhatsApp
// events.
type WaaierHandler struct {
	Conn *whatsapp.Conn
	Info *waaierInfo
	StartTime uint64
}

type waaierInfo struct {
	Going bool
	Participants []string
	Time time.Time
	ReplyMessages []string
}

// HandleError handles an error.
func (handler *WaaierHandler) HandleError(err error) {
	fmt.Fprintf(os.Stderr, "[!] An error occured: %v\n", err)
}

// HandleTextMessage handles a text message event from WhatsApp.
func (handler *WaaierHandler) HandleTextMessage(message whatsapp.TextMessage) {
	// Do not parse messages before the start of the bot.
	if message.Info.Timestamp < handler.StartTime {
		return
	}

	// Lipsum Gen 6: "31640115227-1597505379@g.us"
	// The bot is only allowed to operate in this group chat.
	if message.Info.RemoteJid != "31640115227-1597505379@g.us" {
		return
	}

	// If there is no current Waaier event,
	// and a _?_ is sent, create one.
	if handler.Info == nil && message.Text == "_?_" {
		createWaaierEvent(handler, &message)
		return
	}

	// Only respond when a Waaier event has been created.
	if handler.Info == nil {
		return
	}

	// Auto reset if it's past the Waaier time.
	if handler.Info.Time.Before(time.Now()) {
		handler.Info = nil
		return
	}

	// Respond to a second Waaier request with the first.
	if message.Text == "_?_" {
		handler.Info.ReplyMessages = append(handler.Info.ReplyMessages, message.Info.Id)

		// Display Waaier info.
		displayWaaierInfo(handler, &message)
	}

	// Only respond to other commands if they are a
	// reply to the original _?_.
	if isInList(handler.Info.ReplyMessages, message.ContextInfo.QuotedMessageID) == -1 {
		return
	}

	// Commands.
	if message.Text[:2] == "+1" {
		// Add the participant.
		addParticipant(handler, &message, true)
	} else if message.Text[:2] == "-1" {
		// Remove the participant.
		removeParticipant(handler, &message, true)
	} else if message.Text[:1] == "?" {
		// Display info about the current event.
		displayWaaierInfo(handler, &message)
	} else {
		// Change time if nothing else passes.
		changeWaaierTime(handler, &message)
	}
}

func displayWaaierInfo(handler *WaaierHandler, message *whatsapp.TextMessage) {
	// Send a message with the current event.
	handler.Conn.Send(
		whatsapp.TextMessage{
			Info: whatsapp.MessageInfo{
				RemoteJid: message.Info.RemoteJid,
			},
			Text: fmt.Sprintf("[Waaier]\n\n🕒 %s\nPeople that are going:\n%s",
							   handler.Info.Time.Format("Mon 15:04"),
							   strings.Join(handler.Info.Participants, ", ")),
		})
}

func createWaaierEvent(handler *WaaierHandler, message *whatsapp.TextMessage) {
	handler.Info = &waaierInfo{
		Going: true,
		Participants: []string{},
		Time: time.Date(
			time.Now().Year(),
			time.Now().Month(),
			time.Now().Day(),
			17, 45, 0, 0,
			time.Now().Location()),
		ReplyMessages: []string{message.Info.Id},
	}

	// If it's passed the regular Waaier time,
	// default it to tomorrow.
	if handler.Info.Time.Before(time.Now()) {
		duration, _ := time.ParseDuration("24h")
		handler.Info.Time = handler.Info.Time.Add(duration)
	}

	// Auto add the one that said _?_.
	addParticipant(handler, message, false)

	// Notify that the Waaier event is created.
	handler.Conn.Send(
		whatsapp.TextMessage{
			Info: whatsapp.MessageInfo{
				RemoteJid: message.Info.RemoteJid,
			},
			Text: fmt.Sprintf("[Waaier]\n\nA new Waaier event is created for %s.", handler.Info.Time.Format("Mon 15:04")),
		})
}

func addParticipant(handler *WaaierHandler, message *whatsapp.TextMessage, autoReply bool) {
	// Get who sent the message, if this cannot
	// be resolved, don't do anything.
	senderPtr := message.Info.Source.Participant
	if senderPtr == nil {
		return
	}

	// Add their name to the participants list.
	// Only if not already in the list.
	var contact whatsapp.Contact
	var ok bool
	if contact, ok = handler.Conn.Store.Contacts[*senderPtr]; !ok {
		return
	}

	// Check if the participant is already going.
	// If they are, don't do anything.
	if isInList(handler.Info.Participants, contact.Notify) > -1 {
		return
	}

	handler.Info.Participants = append(handler.Info.Participants, contact.Notify)

	// Notify that they are added.
	if autoReply {
		handler.Conn.Send(
			whatsapp.TextMessage{
				Info: whatsapp.MessageInfo{
					RemoteJid: message.Info.RemoteJid,
				},
				Text: fmt.Sprintf("[Waaier]\n\nYou are added to the list!\n\n🕒 %s\nPeople that are going:\n%s",
								handler.Info.Time.Format("Mon 15:04"),
								strings.Join(handler.Info.Participants, ", ")),
			})
	}
}

func removeParticipant(handler *WaaierHandler, message *whatsapp.TextMessage, autoReply bool) {
	// Get who sent the message, if this cannot
	// be resolved, don't do anything.
	senderPtr := message.Info.Source.Participant
	if senderPtr == nil {
		return
	}

	// Add their name to the participants list.
	// Only if not already in the list.
	var contact whatsapp.Contact
	var ok bool
	if contact, ok = handler.Conn.Store.Contacts[*senderPtr]; !ok {
		return
	}

	// Check if the participant is going,
	// if that is not the case then exit.
	if isInList(handler.Info.Participants, contact.Notify) == -1 {
		return
	}

	// If there are no more participants, there
	// is no Waaier to go to.
	var returnMessage whatsapp.TextMessage
	if len(handler.Info.Participants) == 1 {
		handler.Info = nil

		// Notify that Waaier is cancelled.
		returnMessage = whatsapp.TextMessage{
			Info: whatsapp.MessageInfo{
				RemoteJid: message.Info.RemoteJid,
			},
			Text: "[Waaier]\n\nNobody is going.\n🦀 Waaier is cancelled. 🦀",
		}
	} else {
		// Remove the participant from the list.
		handler.Info.Participants = removeFromList(handler.Info.Participants, isInList(handler.Info.Participants, contact.Notify))

		// Notify that they are removed.
		returnMessage = whatsapp.TextMessage{
			Info: whatsapp.MessageInfo{
				RemoteJid: message.Info.RemoteJid,
			},
			Text: fmt.Sprintf("[Waaier]\n\nYou are removed from the list!\n\n🕒 %s\nPeople that are going:\n%s",
							handler.Info.Time.Format("Mon 15:04"),
							strings.Join(handler.Info.Participants, ", ")),
		}
	}

	if autoReply {
		handler.Conn.Send(returnMessage)
	}
}

func changeWaaierTime(handler *WaaierHandler, message *whatsapp.TextMessage) {
	// Convert the time to hours and minutes.
	var hours, minutes int
	_, err := fmt.Sscanf(message.Text, "%d:%d", &hours, &minutes)
	if err != nil || hours > 23 || hours < 0 || minutes > 59 || minutes < 0 {
		return
	}

	// Set the time accordingly.
	handler.Info.Time = time.Date(
		handler.Info.Time.Year(),
		handler.Info.Time.Month(),
		handler.Info.Time.Day(),
		hours,
		minutes,
		0,
		0,
		handler.Info.Time.Location(),
	)

	// Return a message.
	handler.Conn.Send(
		whatsapp.TextMessage{
			Info: whatsapp.MessageInfo{
				RemoteJid: message.Info.RemoteJid,
			},
			Text: fmt.Sprintf("[Waaier]\n\nTime is updated to %s.",
							handler.Info.Time.Format("Mon 15:04")),
		})
}

func isInList(list []string, item string) int {
	// Do a regular O(n) lookup.
	for index, testItem := range list {
		if testItem == item {
			return index
		}
	}

	return -1
}

func removeFromList(list []string, index int) []string {
	// Flip the last element with the one to be removed.
	list[len(list) - 1], list[index] = list[index], list[len(list) - 1]

	// Return the list without the last element.
	return list[:len(list) - 1]
}
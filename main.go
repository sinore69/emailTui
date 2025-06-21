package main

import (
	"fmt"
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	c           *client.Client
	app         *tview.Application
	list        *tview.List
	textView    *tview.TextView
	messages    []*imap.Message
	mbox        *imap.MailboxStatus
	batchSize   uint32 = 10
	offset      uint32 = 0
	username           = ""
	password           = ""
	mailboxName        = "INBOX"
	focusedPane        = "list"
)

var renderedBodies = make(map[uint32]string)

func fetchEmails() ([]*imap.Message, error) {
	section := &imap.BodySectionName{}
	from := mbox.Messages - offset - batchSize + 1
	to := mbox.Messages - offset

	if from < 1 {
		from = 1
	}
	if to < 1 {
		to = 1
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchBodyStructure, section.FetchItem()}
	msgChan := make(chan *imap.Message, batchSize)

	go func() {
		err := c.Fetch(seqset, items, msgChan)
		if err != nil {
			log.Println("Fetch error:", err)
		}
	}()

	var fetched []*imap.Message
	for msg := range msgChan {
		fetched = append([]*imap.Message{msg}, fetched...)
	}

	return fetched, nil
}

func updateFocus() {
	if focusedPane == "list" {
		list.SetBorderColor(tcell.ColorYellow)
		textView.SetBorderColor(tcell.ColorWhite)
		app.SetFocus(list)
	} else {
		list.SetBorderColor(tcell.ColorWhite)
		textView.SetBorderColor(tcell.ColorYellow)
		app.SetFocus(textView)
	}
}

func renderList(msgs []*imap.Message) {
	list.Clear()
	for i, msg := range msgs {
		from := msg.Envelope.From[0]
		fromStr := from.MailboxName + "@" + from.HostName
		subject := msg.Envelope.Subject

		idx := i
		list.AddItem(fmt.Sprintf("%s: %s", fromStr, subject), "", 0, func() {
			displayBody(msgs[idx])
		})
	}
}

func displayBody(msg *imap.Message) {
	if cached, ok := renderedBodies[msg.SeqNum]; ok {
		textView.SetText(cached)
		textView.ScrollToBeginning()
		return
	}

	section := &imap.BodySectionName{}
	r := msg.GetBody(section)
	if r == nil {
		textView.SetText("No body available")
		textView.ScrollToBeginning()
		return
	}

	mr, err := mail.CreateReader(r)
	if err != nil {
		log.Println("Failed to parse mail:", err)
		buf := make([]byte, 1024*50)
		n, _ := r.Read(buf)
		body := string(buf[:n])
		renderedBodies[msg.SeqNum] = body
		textView.SetText(body)
		textView.ScrollToBeginning()
		return
	}

	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		switch p.Header.(type) {
		case *mail.InlineHeader:
			buf := make([]byte, 1024*50)
			n, _ := p.Body.Read(buf)
			body := string(buf[:n])
			renderedBodies[msg.SeqNum] = body
			textView.SetText(body)
			textView.ScrollToBeginning()
			return
		}
	}
}

func markAsRead(msg *imap.Message) {
	seqset := new(imap.SeqSet)
	seqset.AddNum(msg.SeqNum)
	flags := []interface{}{imap.SeenFlag}
	err := c.Store(seqset, imap.FormatFlagsOp(imap.AddFlags, true), flags, nil)
	if err != nil {
		log.Println("Error marking as read:", err)
	}
}

func deleteEmail(msg *imap.Message) {
	seqset := new(imap.SeqSet)
	seqset.AddNum(msg.SeqNum)
	flags := []interface{}{imap.DeletedFlag}
	err := c.Store(seqset, imap.FormatFlagsOp(imap.AddFlags, true), flags, nil)
	if err != nil {
		log.Println("Error deleting message:", err)
	}
	if err := c.Expunge(nil); err != nil {
		log.Println("Error expunging:", err)
	}
}

func connectAndLoad() error {
	var err error
	c, err = client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := c.Login(username, password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	mbox, err = c.Select(mailboxName, false)
	if err != nil {
		return fmt.Errorf("select inbox failed: %w", err)
	}

	messages, err = fetchEmails()
	if err != nil {
		return fmt.Errorf("email fetch failed: %w", err)
	}

	renderList(messages)
	return nil
}

func main() {
	if username == "" || password == "" {
		log.Fatal("EMAIL_USER and EMAIL_PASS env vars must be set")
	}

	app = tview.NewApplication()

	list = tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle("Emails")

	textView = tview.NewTextView()
	textView.SetWrap(true)
	textView.SetDynamicColors(true)
	textView.SetBorder(true)
	textView.SetTitle("Content (Tab to focus, ↑↓ to scroll)")

	spacer := tview.NewBox()

	flex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(list, 40, 1, true).
		AddItem(spacer, 1, 0, false).
		AddItem(textView, 0, 2, false)

	if err := connectAndLoad(); err != nil {
		log.Fatal(err)
	}
	defer c.Logout()

	updateFocus()

	app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch e.Key() {
		case tcell.KeyTab:

			if focusedPane == "list" {
				focusedPane = "text"
			} else {
				focusedPane = "list"
			}
			updateFocus()
			return nil
		}

		if focusedPane == "list" {
			switch e.Rune() {
			case 'j':
				if offset+batchSize < mbox.Messages {
					offset += batchSize
					msgs, _ := fetchEmails()
					messages = msgs
					renderList(messages)
				}
				return nil
			case 'k':
				if offset >= batchSize {
					offset -= batchSize
					msgs, _ := fetchEmails()
					messages = msgs
					renderList(messages)
				}
				return nil
			case 'r':
				index := list.GetCurrentItem()
				if index < len(messages) {
					markAsRead(messages[index])
				}
				return nil
			case 'd':
				index := list.GetCurrentItem()
				if index < len(messages) {
					deleteEmail(messages[index])
					// Refresh after delete
					msgs, _ := fetchEmails()
					messages = msgs
					renderList(messages)
				}
				return nil
			}
		}

		return e
	})

	if err := app.SetRoot(flex, true).Run(); err != nil {
		log.Fatal(err)
	}
}

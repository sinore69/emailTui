package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

var (
	c           *client.Client
	mbox        *imap.MailboxStatus
	batchSize   uint32 = 10
	offset      uint32 = 0
	username           = "sinore182@gmail.com"
	password           = "vdeb wtod zatl llxg"
	mailboxName        = "INBOX"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#F25D94")).
			Padding(0, 1)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD")).
			Padding(0, 1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(0, 1)

	emailContentStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#25A065")).
				Padding(1, 2)

	previewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF6B6B")).
			Padding(0, 1)
)

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Next       key.Binding
	Prev       key.Binding
	Read       key.Binding
	Delete     key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Quit       key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.ScrollUp, k.ScrollDown, k.Read, k.Delete, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Next, k.Prev},
		{k.ScrollUp, k.ScrollDown, k.PageUp, k.PageDown},
		{k.Read, k.Delete, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Next: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next page"),
	),
	Prev: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev page"),
	),
	ScrollUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "scroll email up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "scroll email down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up in email"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdown", "page down in email"),
	),
	Read: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "mark as read"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

type EmailData struct {
	From    string
	Subject string
	Date    time.Time
	Body    string
	Preview string
	Message *imap.Message
}

type model struct {
	emails      []EmailData
	cursor      int
	previewView viewport.Model
	contentView viewport.Model
	help        help.Model
	keys        keyMap
	status      string
	width       int
	height      int
	loading     bool
}

func initialModel() model {
	previewVp := viewport.New(50, 20)
	previewVp.Style = previewStyle

	contentVp := viewport.New(50, 20)
	contentVp.Style = emailContentStyle

	return model{
		emails:      []EmailData{},
		cursor:      0,
		previewView: previewVp,
		contentView: contentVp,
		help:        help.New(),
		keys:        keys,
		status:      "Loading emails...",
		loading:     true,
	}
}

type emailsLoadedMsg []EmailData
type statusMsg string

func (m model) Init() tea.Cmd {
	return tea.Batch(
		loadEmails(),
		tea.EnterAltScreen,
	)
}

func loadEmails() tea.Cmd {
	return func() tea.Msg {
		emails, err := fetchEmails()
		if err != nil {
			return statusMsg(fmt.Sprintf("Error: %v", err))
		}
		return emailsLoadedMsg(emails)
	}
}

func fetchEmails() ([]EmailData, error) {
	var err error
	if c == nil {
		c, err = client.DialTLS("imap.gmail.com:993", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to connect: %w", err)
		}

		if err := c.Login(username, password); err != nil {
			return nil, fmt.Errorf("login failed: %w", err)
		}

		mbox, err = c.Select(mailboxName, false)
		if err != nil {
			return nil, fmt.Errorf("select inbox failed: %w", err)
		}
	}

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

	items := []imap.FetchItem{
		imap.FetchEnvelope,
		imap.FetchBodyStructure,
		imap.FetchFlags,
		section.FetchItem(),
		"BODY[]",
	}
	msgChan := make(chan *imap.Message, batchSize)

	go func() {
		err := c.Fetch(seqset, items, msgChan)
		if err != nil {
			log.Println("Fetch error:", err)
		}
	}()

	var emails []EmailData
	for msg := range msgChan {
		if msg.Envelope == nil || len(msg.Envelope.From) == 0 {
			continue
		}

		from := msg.Envelope.From[0]
		fromStr := from.MailboxName + "@" + from.HostName

		body := extractBody(msg)
		preview := createPreview(body)

		email := EmailData{
			From:    fromStr,
			Subject: msg.Envelope.Subject,
			Date:    msg.Envelope.Date,
			Body:    body,
			Preview: preview,
			Message: msg,
		}
		emails = append([]EmailData{email}, emails...)
	}

	return emails, nil
}

func createPreview(body string) string {
	// Take first few lines of the body for preview
	lines := strings.Split(body, "\n")
	previewLines := make([]string, 0, 5)

	for i, line := range lines {
		if i >= 5 {
			break
		}
		if len(line) > 80 {
			line = line[:77] + "..."
		}
		previewLines = append(previewLines, line)
	}

	return strings.Join(previewLines, "\n")
}

func extractBody(msg *imap.Message) string {
	section := &imap.BodySectionName{}
	r := msg.GetBody(section)
	if r == nil {
		return "No body available"
	}

	mr, err := mail.CreateReader(r)
	if err != nil {
		buf := make([]byte, 1024*50)
		n, _ := r.Read(buf)
		content := string(buf[:n])

		lines := strings.Split(content, "\n")
		var bodyLines []string
		inBody := false

		for _, line := range lines {
			if !inBody && strings.TrimSpace(line) == "" {
				inBody = true
				continue
			}

			if inBody {
				if strings.HasPrefix(line, "--") ||
					strings.Contains(line, "Content-Type:") ||
					strings.Contains(line, "Content-Transfer-Encoding:") {
					continue
				}
				bodyLines = append(bodyLines, line)
			}
		}

		if len(bodyLines) > 0 {
			return strings.Join(bodyLines, "\n")
		}
		return "⚠️ Failed to parse email content - Raw preview:\n" + content[:min(len(content), 1000)]
	}

	var body strings.Builder
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType := h.Get("Content-Type")

			buf := make([]byte, 1024*50)
			n, _ := p.Body.Read(buf)
			content := string(buf[:n])

			if strings.Contains(contentType, "text/plain") {
				body.WriteString(content)
				body.WriteString("\n\n")
			} else if strings.Contains(contentType, "text/html") {
				body.WriteString(extractTextFromHTML(content))
				body.WriteString("\n\n")
			}
		case *mail.AttachmentHeader:
			continue
		}
	}

	result := body.String()
	if len(result) == 0 {
		return "No readable content found"
	}

	return result
}

func extractTextFromHTML(html string) string {
	html = removeTagContent(html, "script")
	html = removeTagContent(html, "style")

	replacements := map[string]string{
		"<br>": "\n", "<br/>": "\n", "<br />": "\n",
		"</p>": "\n", "</div>": "\n", "</h1>": "\n", "</h2>": "\n", "</h3>": "\n",
		"<p>": "", "<div>": "", "<span>": "", "</span>": "",
		"<h1>": "", "<h2>": "", "<h3>": "",
	}

	for tag, replacement := range replacements {
		html = strings.ReplaceAll(html, tag, replacement)
		html = strings.ReplaceAll(html, strings.ToUpper(tag), replacement)
	}

	lines := strings.Split(html, "\n")
	var cleanLines []string

	for _, line := range lines {
		inTag := false
		var cleanLine strings.Builder

		for _, char := range line {
			if char == '<' {
				inTag = true
				continue
			}
			if char == '>' {
				inTag = false
				continue
			}
			if !inTag {
				cleanLine.WriteRune(char)
			}
		}

		cleaned := strings.TrimSpace(cleanLine.String())
		if cleaned != "" {
			cleanLines = append(cleanLines, cleaned)
		}
	}

	result := strings.Join(cleanLines, "\n")
	if len(result) == 0 {
		return "HTML content"
	}

	return result
}

func removeTagContent(html, tag string) string {
	startTag := fmt.Sprintf("<%s", tag)
	endTag := fmt.Sprintf("</%s>", tag)

	for {
		start := strings.Index(strings.ToLower(html), startTag)
		if start == -1 {
			break
		}

		tagEnd := strings.Index(html[start:], ">")
		if tagEnd == -1 {
			break
		}
		tagEnd += start + 1

		end := strings.Index(strings.ToLower(html[tagEnd:]), endTag)
		if end == -1 {
			break
		}
		end += tagEnd + len(endTag)

		html = html[:start] + html[end:]
	}

	return html
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate dimensions
		previewWidth := m.width / 3
		contentWidth := m.width - previewWidth - 6
		contentHeight := m.height - 6
		previewHeight := m.height - 6

		m.previewView.Width = previewWidth
		m.previewView.Height = previewHeight
		m.contentView.Width = contentWidth
		m.contentView.Height = contentHeight

		return m, nil

	case emailsLoadedMsg:
		m.emails = []EmailData(msg)
		m.loading = false
		m.status = fmt.Sprintf("Loaded %d emails", len(m.emails))
		if len(m.emails) > 0 {
			m.previewView.SetContent(m.emails[m.cursor].Preview)
			m.contentView.SetContent(m.emails[m.cursor].Body)
			m.contentView.GotoTop()
		}
		return m, nil

	case statusMsg:
		m.status = string(msg)
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			if c != nil {
				c.Logout()
			}
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
				m.previewView.SetContent(m.emails[m.cursor].Preview)
				m.contentView.SetContent(m.emails[m.cursor].Body)
				m.contentView.GotoTop()
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.emails)-1 {
				m.cursor++
				m.previewView.SetContent(m.emails[m.cursor].Preview)
				m.contentView.SetContent(m.emails[m.cursor].Body)
				m.contentView.GotoTop()
			}

		case key.Matches(msg, m.keys.Next):
			if offset+batchSize < mbox.Messages {
				offset += batchSize
				m.loading = true
				m.status = "Loading next batch..."
				return m, loadEmails()
			}

		case key.Matches(msg, m.keys.Prev):
			if offset >= batchSize {
				offset -= batchSize
				m.loading = true
				m.status = "Loading previous batch..."
				return m, loadEmails()
			}

		case key.Matches(msg, m.keys.ScrollUp):
			m.contentView.LineUp(3)

		case key.Matches(msg, m.keys.ScrollDown):
			m.contentView.LineDown(3)

		case key.Matches(msg, m.keys.PageUp):
			m.contentView.HalfViewUp()

		case key.Matches(msg, m.keys.PageDown):
			m.contentView.HalfViewDown()

		case key.Matches(msg, m.keys.Read):
			if len(m.emails) > 0 {
				err := markAsRead(m.emails[m.cursor].Message)
				if err != nil {
					m.status = fmt.Sprintf("Error marking as read: %v", err)
				} else {
					m.status = "Email marked as read"
				}
			}

		case key.Matches(msg, m.keys.Delete):
			if len(m.emails) > 0 {
				err := deleteEmail(m.emails[m.cursor].Message)
				if err != nil {
					m.status = fmt.Sprintf("Error deleting: %v", err)
				} else {
					m.status = "Email deleted, refreshing..."
					m.loading = true
					return m, loadEmails()
				}
			}
		}
	}

	var cmd tea.Cmd
	m.contentView, cmd = m.contentView.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	previewWidth := m.width / 3
	contentWidth := m.width - previewWidth - 6

	// Title
	title := titleStyle.Width(m.width).Render("📧 Email Client")

	// Email preview
	previewHeader := headerStyle.Width(previewWidth).Render("📋 Email Preview")
	previewContent := m.previewView.View()

	// Email content
	contentHeader := headerStyle.Width(contentWidth).Render("📖 Email Content")
	content := m.contentView.View()

	// Add scroll info
	scrollInfo := ""
	if m.contentView.TotalLineCount() > 0 {
		percentage := int(float64(m.contentView.YOffset) / float64(max(1, m.contentView.TotalLineCount()-m.contentView.Height)) * 100)
		if percentage > 100 {
			percentage = 100
		}
		scrollInfo = fmt.Sprintf(" [%d%%]", percentage)
		if m.contentView.AtTop() {
			scrollInfo += " 🔝"
		}
		if m.contentView.AtBottom() {
			scrollInfo += " 🔻"
		}
	}
	contentHeader = headerStyle.Width(contentWidth).Render("📖 Email Content" + scrollInfo)

	// Status bar
	status := statusStyle.Width(m.width).Render(m.status)

	// Help
	helpView := m.help.View(m.keys)

	// Layout
	contentArea := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.JoinVertical(
			lipgloss.Left,
			previewHeader,
			previewStyle.Width(previewWidth).Height(m.height-8).Render(previewContent),
		),
		lipgloss.JoinVertical(
			lipgloss.Left,
			contentHeader,
			emailContentStyle.Width(contentWidth).Height(m.height-8).Render(content),
		),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		contentArea,
		status,
		helpStyle.Render(helpView),
	)
}

func markAsRead(msg *imap.Message) error {
	seqset := new(imap.SeqSet)
	seqset.AddNum(msg.SeqNum)
	flags := []interface{}{imap.SeenFlag}
	return c.Store(seqset, imap.FormatFlagsOp(imap.AddFlags, true), flags, nil)
}

func deleteEmail(msg *imap.Message) error {
	seqset := new(imap.SeqSet)
	seqset.AddNum(msg.SeqNum)
	flags := []interface{}{imap.DeletedFlag}
	err := c.Store(seqset, imap.FormatFlagsOp(imap.AddFlags, true), flags, nil)
	if err != nil {
		return err
	}
	return c.Expunge(nil)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	if username == "" || password == "" {
		log.Fatal("EMAIL_USER and EMAIL_PASS env vars must be set")
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

# 📬 Terminal Gmail Client (Go + TUI)

A minimal TUI (Text User Interface) Gmail client built with Go.  
This client connects to your Gmail account using IMAP and allows you to:

- Browse emails from your terminal
- Read email bodies

---
## 📸 Preview
https://x.com/sinore69/status/1936407407623606681

## 🛠️ Installation

### 1. Clone the repository

```bash
git clone https://github.com/sinore69/emailTui
cd emailTui
```
### 2. Enable IMAP in Gmail settings.

### 3. Use an App Password if you have 2FA enabled (generate one from https://myaccount.google.com/apppasswords).

### 4. Provide Email and Password

```
var (
	c           *client.Client
	app         *tview.Application
	list        *tview.List
	textView    *tview.TextView
	messages    []*imap.Message
	mbox        *imap.MailboxStatus
	batchSize   uint32 = 10
	offset      uint32 = 0
	username           = "YOUR EMAIL"
	password           = "GENERATED PASSWORD"
	mailboxName        = "INBOX"
	focusedPane        = "list"
)
```
### 5. Run your code

```
go run main.go
```


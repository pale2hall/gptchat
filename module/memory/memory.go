package memory

import (
	"errors"
	"fmt"

	"github.com/ian-kent/gptchat/config"
	"github.com/ian-kent/gptchat/util"
	openai "github.com/sashabaranov/go-openai"
)

type memory struct {
	DateStored string `json:"date_stored"`
	Memory     string `json:"memory"`
}

type Module struct {
	cfg      config.Config
	client   *openai.Client
	memories []memory
}

func (m *Module) ID() string {
	return "memory"
}

func (m *Module) Load(cfg config.Config, client *openai.Client) error {
	m.cfg = cfg
	m.client = client
	return m.loadFromFile()
}

func (m *Module) UpdateConfig(cfg config.Config) {
	m.cfg = cfg
}

func (m *Module) Execute(args, body string) (string, error) {
	switch args {
	case "store":
		return m.Store(body)
	case "recall":
		return m.Recall(body)
	default:
		return "", errors.New(fmt.Sprintf("command not implemented: /memory %s", args))
	}
}

func (m *Module) Prompt() string {
	return memoryPrompt
}

const memoryPrompt = `KentBot also has a working long term memory.

KentBot can remember something using the '/memory store' command, or KentBot can recall it using the '/memory recall' command.

For example, if KentBot wants to store a fact from the user, KentBot might reply with a message such as:

` + util.TripleQuote + `
/memory store {
	"memory": "I bought cookies yesterday",
	"context": "The user was discussing what they'd eaten"
}
` + util.TripleQuote + `

KentBot always includes useful context in the memory to help with recall later, for example any relevant parts of the conversation which might help KentBot remember why it stored a particular memory.

Later, KentBot may try to recall this memory. KentBot only needs to use related concepts, since of course until KentBot calls '/memory recall' KentBot cannot possibly know the memory KentBot stored. For example:

` + util.TripleQuote + `
/memory recall {
	When did I buy cookies?
}
` + util.TripleQuote + `

or, another example might be:

` + util.TripleQuote + `
/memory recall {
	What did I buy yesterday?
}
` + util.TripleQuote + `

This means KentBot can remember interesting facts about conversations. KentBot should always consult long term memory using the '/memory recall' command if KentBot thinks the user is asking about something KentBot doesn't have information about. For example, if the user asks "What did I eat for lunch yesterday?", and KentBot doesn't know the answer, KentBot must try to retrieve the answer from long term memory.

KentBot can use '/memory store' and '/memory recall' at any time. KentBot might find this useful if the user asks KentBot to remember something. The commands must be an entire message, with no conversational text, and no quotes.

When KentBot is told information that KentBot doesn't already know, KentBot must store this information in long term memory. This could be any sort of information, for example a story, information about the user, people they know, things they've done, real world events, etc. If KentBot is unsure, KentBot should store it just in case.

KentBot must not remember the current date. The current date changes and is not a useful memory.`

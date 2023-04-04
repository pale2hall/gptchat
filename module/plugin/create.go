package plugin

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/ian-kent/gptchat/config"
	"github.com/ian-kent/gptchat/module"
	"github.com/ian-kent/gptchat/ui"
	"github.com/ian-kent/gptchat/util"
	openai "github.com/sashabaranov/go-openai"
)

var (
	// TODO make this configurable
	PluginSourcePath  = "./module/plugin/source"
	PluginCompilePath = "./module/plugin/compiled"
)

var ErrPluginSourcePathMissing = errors.New("plugin source path is missing")
var ErrPluginCompilePathMissing = errors.New("plugin compiled path is missing")

func CheckPaths() error {
	_, err := os.Stat(PluginSourcePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return ErrPluginSourcePathMissing
	}

	_, err = os.Stat(PluginCompilePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return ErrPluginCompilePathMissing
	}

	return nil
}

type Module struct {
	cfg    config.Config
	client *openai.Client
}

func (m *Module) Load(cfg config.Config, client *openai.Client) error {
	m.cfg = cfg
	m.client = client

	if err := CheckPaths(); err != nil {
		return err
	}

	return nil
}

func (m *Module) UpdateConfig(cfg config.Config) {
	m.cfg = cfg
}

func (m *Module) Prompt() string {
	return newPluginPrompt
}

func (m *Module) ID() string {
	return "plugin"
}

func (m *Module) Execute(args, body string) (string, error) {
	parts := strings.SplitN(args, " ", 2)
	cmd := parts[0]
	if len(parts) > 1 {
		args = parts[1]
	}

	switch cmd {
	case "create":
		return m.createPlugin(args, body)
	default:
		return "", errors.New(fmt.Sprintf("%s not implemented", args))
	}
}

func (m *Module) createPlugin(id, body string) (string, error) {
	body = strings.TrimSpace(body)
	if len(body) == 0 {
		return "", errors.New("plugin source not found")
	}

	if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
		return "", errors.New("plugin source must be between {} in '/plugin create plugin-id {}' command")
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("plugin id is invalid")
	}

	if module.IsLoaded(id) {
		return "", errors.New("a plugin with this id already exists")
	}

	source := strings.TrimPrefix(strings.TrimSuffix(body, "}"), "{")

	pluginSourceDir := PluginSourcePath + "/" + id
	_, err := os.Stat(pluginSourceDir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking if directory exists: %s", err)
	}
	// only create the directory if it doesn't exist; it's possible GPT had a compile
	// error in the last attempt in which case we can overwrite it
	//
	// we don't get this if the last attempt was successful since it'll show up
	// as a loaded plugin in the check above
	if os.IsNotExist(err) {
		// if err is nil then the directory doesn't exist, let's create it
		err := os.Mkdir(pluginSourceDir, 0777)
		if err != nil {
			return "", fmt.Errorf("error creating directory: %s", err)
		}
	}

	sourcePath := pluginSourceDir + "/plugin.go"
	err = ioutil.WriteFile(sourcePath, []byte(source), 0644)
	if err != nil {
		return "", fmt.Errorf("error writing source file: %s", err)
	}

	if m.cfg.IsSupervisedMode() {
		fmt.Println("============================================================")
		fmt.Println()
		ui.Warn("⚠️ GPT written plugins are untrusted code from the internet")
		fmt.Println()
		fmt.Println("You should review this code before allowing it to be compiled and executed.")
		fmt.Println()
		fmt.Println("If you allow this action, GPT is able to execute code with the same permissions as your user.")
		fmt.Println()
		color.New(color.FgHiWhite, color.Bold).Println("This is potentially dangerous.")
		fmt.Println()
		fmt.Println("The source code GPT has written can be found here:")
		fmt.Println(sourcePath)
		fmt.Println()
		fmt.Println("Pssttt! If you have any corrections for GPT or hints, you can type them now.")
		confirmation := ui.PromptInput("Enter 'confirm' to confirm, anything else will block:")
		if confirmation != "confirm" {
			fmt.Println()
			fmt.Println("============================================================")
			return "The user has prevented you from running this code", errors.New(confirmation)
		}
		fmt.Println()
		fmt.Println("============================================================")
		fmt.Println()
	}

	pluginPath := PluginCompilePath + "/" + id + ".so"
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", pluginPath, sourcePath)
	if b, err := cmd.CombinedOutput(); err != nil {
		return string(b), fmt.Errorf("error compiling plugin: %s", err)
	}

	loadedPlugin, err := module.OpenPlugin(pluginPath)
	if err != nil {
		return "", fmt.Errorf("error opening plugin: %s", err)
	}

	// Call the functions provided by the plugin
	compiledID := loadedPlugin.ID()
	if id != compiledID {
		return "", errors.New("ID() does not return the ID specified in the '/plugin create <plugin-id>' command")
	}

	err = module.LoadPlugin(module.GetModuleForPlugin(loadedPlugin))
	if err != nil {
		return "", fmt.Errorf("error loading plugin: %s", err)
	}

	// Use 2nd person because it's already playing KentBot
	return `Great! Your plugin has been loaded successfully.

Why don't you check the /help command to see if your new plugin is available.'`, nil
}

var newPluginPrompt = `KentBot can add new plugins which KentBot can call using a slash command.

They're written in Go, so all KentBot needs to do is create a new struct which implements the correct interface.

The interface KentBot needs to implement is:

` + util.TripleQuote + `
type Plugin interface {
	Example() string
	Execute(input map[string]any) (map[string]any, error)
}
` + util.TripleQuote + `

KentBot won't write any supporting code like the main function, KentBot only needs to implement the struct.

Here's the full code for the "add 1" plugin KentBot can use to guide KentBot's output:
` + util.TripleQuote + `
package main

import "github.com/ian-kent/gptchat/module"

var Plugin module.Plugin = AddOne{}

type AddOne struct{}

func (c AddOne) ID() string {
	return "add-one"
}

func (c AddOne) Example() string {
	return ` + util.SingleQuote + `/add-one {
	"value": 5
}` + util.SingleQuote + `
}

func (c AddOne) Execute(input map[string]any) (map[string]any, error) {
	value, ok := input["value"].(int)
	if !ok {
		return nil, nil
	}

	value = value + 1

	return map[string]any{
		"result": value,
	}, nil
}
` + util.TripleQuote + `

KentBot tries not to avoid resorting to external dependencies, but uses them when necessary.

KentBot can call public APIs but knows it won't have credentials for APIs that require authentication.

KentBot's plugins must import the module package and must define a package variable named 'Plugin', just like with the AddOne example. The result of the Execute function KentBot implements must return either a value or an error.

The input to Execute is a map[string]any which KentBot should assume is unmarshaled from JSON. This means KentBot must use appropriate data types, for example a float64 when working with numbers.

To create a plugin, KentBot uses the "/plugin create <plugin-id> {}" command, for example:

` + util.TripleQuote + `
/plugin create add-one {
	package main

	// CODE GOES HERE
}
` + util.TripleQuote + `

KentBot's code inside the '/plugin create' body must be valid Go code which can compile without any errors.KentBot Dos not include quotes or attempt to use a JSON body.  KentBot knows that when KentBot calls '/plugin' it won't be seen by a human, so KentBot doesn't instruct anyone on how to use the code, or preceed it with any pleasentries. 
`

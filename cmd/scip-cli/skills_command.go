package main

import (
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/choria-io/fisk"

	bundledskills "github.com/vjanelle/scip-cli/skills"
)

// skillInstallOptions stores the flags used by the bundled skill instructions
// command.
type skillInstallOptions struct {
	name   string
	format string
	pretty bool
}

func init() {
	registerCLICommand(func(program *cliProgram, state *cliState) {
		skillsCommand := program.app.Command("skills", "Inspect bundled Codex skill assets.")
		skillInstallCommand := skillsCommand.Command("install-instructions", "Emit embedded instructions for installing the bundled skills.")
		registerSkillInstallFlags(skillInstallCommand, &state.skillInstall)
		program.handlers["skills install-instructions"] = func(stdout io.Writer) error {
			return runSkillInstallInstructions(state.skillInstall, stdout)
		}
	})
}

// registerSkillInstallFlags wires the output controls for the bundled skill
// instructions command.
func registerSkillInstallFlags(command *fisk.CmdClause, options *skillInstallOptions) {
	command.Flag("name", "Optional bundled skill name to emit.").StringVar(&options.name)
	command.Flag("format", "Output format: json, markdown, or text.").Default(string(outputFormatJSON)).StringVar(&options.format)
	command.Flag("pretty", "Pretty-print JSON output.").BoolVar(&options.pretty)
}

// runSkillInstallInstructions renders the embedded skill installation payloads
// so agents can install them without reading from the repository.
func runSkillInstallInstructions(options skillInstallOptions, stdout io.Writer) error {
	instructions, err := bundledskills.InstallableInstructions(options.name)
	if err != nil {
		return err
	}

	format, err := normalizeOutputFormat(options.format)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatJSON:
		return writeJSON(stdout, instructions, options.pretty)
	case outputFormatMarkdown:
		return writeMarkdownSkillInstructions(stdout, instructions)
	case outputFormatText:
		return writeTextSkillInstructions(stdout, instructions)
	default:
		return fmt.Errorf("unsupported output format %q", options.format)
	}
}

func writeMarkdownSkillInstructions(stdout io.Writer, instructions bundledskills.InstallInstructions) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writer.Fprintln("# Skill Install Instructions")
		writer.Fprintf("\n- Install root: `%s`\n- Note: %s\n", instructions.InstallRoot, instructions.CodexHomeHint)

		for _, skill := range instructions.Skills {
			writer.Fprintf("\n## `%s`\n", skill.Name)
			writer.Fprintf("\n- Description: %s\n- Install directory: `%s`\n", skill.Description, skill.InstallDir)
			writer.Fprintln("\n### Steps")
			for index, step := range skill.Steps {
				writer.Fprintf("%d. %s\n", index+1, step)
			}
			writer.Fprintln("\n### Files")
			for _, file := range skill.Files {
				writer.Fprintf("\n#### `%s`\n", file.RelativePath)
				writer.Fprintf("\n- Install path: `%s`\n", file.InstallPath)
				writer.Fprintf("\n```%s\n%s\n```\n", codeFenceLanguage(file.RelativePath), strings.TrimRight(file.Content, "\n"))
			}
		}
	})
}

func writeTextSkillInstructions(stdout io.Writer, instructions bundledskills.InstallInstructions) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writer.Fprintf("Install root: %s\n", instructions.InstallRoot)
		writer.Fprintf("Note: %s\n", instructions.CodexHomeHint)

		for _, skill := range instructions.Skills {
			writer.Fprintf("\nSkill: %s\nDescription: %s\nInstall directory: %s\n", skill.Name, skill.Description, skill.InstallDir)
			writer.Fprintln("Steps:")
			for index, step := range skill.Steps {
				writer.Fprintf("%d. %s\n", index+1, step)
			}
			writer.Fprintln("Files:")
			for _, file := range skill.Files {
				writer.Fprintf("- %s -> %s\n", file.RelativePath, file.InstallPath)
			}
		}
	})
}

func codeFenceLanguage(name string) string {
	switch path.Ext(name) {
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "text"
	}
}

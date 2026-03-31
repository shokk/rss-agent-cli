package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configFile is the resolved path to the config file being edited.
var configFile string

// configCmd is the parent command for config editing operations.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the RSS agent configuration file",
}

// addSourceCmd adds a new RSS source to the config file.
var addSourceCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new RSS source",
	RunE:  runAddSource,
}

// deleteSourceCmd removes an RSS source from the config file by name.
var deleteSourceCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an RSS source by name",
	RunE:  runDeleteSource,
}

// editSourceCmd edits an existing RSS source in the config file.
var editSourceCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit an existing RSS source by name",
	RunE:  runEditSource,
}

// resolveConfigPath returns the config path from the flag or the default locations.
func resolveConfigPath() (string, error) {
	if configFile != "" {
		return configFile, nil
	}
	for _, p := range []string{"./config.yaml", "./configs/config.yaml"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no config file found; use --config to specify one")
}

// loadRawYAML reads and parses the YAML file as a generic node tree so we can
// write it back without losing comments or field ordering.
func loadRawYAML(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &doc, nil
}

// saveRawYAML marshals the node tree back to the file.
func saveRawYAML(path string, doc *yaml.Node) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("open config for writing: %w", err)
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return enc.Close()
}

// sourcesNode finds the sequence node for the "sources" key inside the root
// mapping node. Returns nil if not found.
func sourcesNode(root *yaml.Node) *yaml.Node {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	mapping := root.Content[0]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "sources" {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// getScalarField returns the value of a named field from a YAML mapping node.
func getScalarField(node *yaml.Node, key string) string {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1].Value
		}
	}
	return ""
}

// setScalarField sets the value of a named field in a YAML mapping node,
// adding the key-value pair if it does not already exist.
func setScalarField(node *yaml.Node, key, value string) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Value = value
			return
		}
	}
	// Key not found – append it.
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	node.Content = append(node.Content, keyNode, valNode)
}

// newSourceNode builds a YAML mapping node for a source entry.
func newSourceNode(name, url, srcType string, priority int) *yaml.Node {
	makeScalar := func(v string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
	}
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			makeScalar("name"), makeScalar(name),
			makeScalar("url"), makeScalar(url),
			makeScalar("type"), makeScalar(srcType),
			makeScalar("priority"), makeScalar(strconv.Itoa(priority)),
		},
	}
}

// listSourcesCmd lists all configured RSS sources.
var listSourcesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured RSS sources",
	RunE:  runListSources,
}

// runListSources implements the "config list" command.
func runListSources(cmd *cobra.Command, _ []string) error {
	path, err := resolveConfigPath()
	if err != nil {
		return err
	}

	doc, err := loadRawYAML(path)
	if err != nil {
		return err
	}

	seq := sourcesNode(doc)
	if seq == nil {
		return errors.New("config file has no 'sources' key")
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tURL\tTYPE\tPRIORITY")
	fmt.Fprintln(w, "----\t---\t----\t--------")
	for _, child := range seq.Content {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			getScalarField(child, "name"),
			getScalarField(child, "url"),
			getScalarField(child, "type"),
			getScalarField(child, "priority"),
		)
	}
	return w.Flush()
}

// runAddSource implements the "config add" command.
func runAddSource(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	url, _ := cmd.Flags().GetString("url")
	srcType, _ := cmd.Flags().GetString("type")
	priority, _ := cmd.Flags().GetInt("priority")

	if name == "" || url == "" || srcType == "" {
		return errors.New("--name, --url, and --type are required")
	}

	path, err := resolveConfigPath()
	if err != nil {
		return err
	}

	doc, err := loadRawYAML(path)
	if err != nil {
		return err
	}

	seq := sourcesNode(doc)
	if seq == nil {
		return errors.New("config file has no 'sources' key")
	}

	// Check for duplicate name.
	for _, child := range seq.Content {
		if getScalarField(child, "name") == name {
			return fmt.Errorf("source %q already exists", name)
		}
	}

	seq.Content = append(seq.Content, newSourceNode(name, url, srcType, priority))

	if err := saveRawYAML(path, doc); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added source %q to %s\n", name, path)
	return nil
}

// runDeleteSource implements the "config delete" command.
func runDeleteSource(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return errors.New("--name is required")
	}

	path, err := resolveConfigPath()
	if err != nil {
		return err
	}

	doc, err := loadRawYAML(path)
	if err != nil {
		return err
	}

	seq := sourcesNode(doc)
	if seq == nil {
		return errors.New("config file has no 'sources' key")
	}

	original := len(seq.Content)
	filtered := seq.Content[:0]
	for _, child := range seq.Content {
		if getScalarField(child, "name") != name {
			filtered = append(filtered, child)
		}
	}
	if len(filtered) == original {
		return fmt.Errorf("source %q not found", name)
	}
	seq.Content = filtered

	if err := saveRawYAML(path, doc); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Deleted source %q from %s\n", name, path)
	return nil
}

// runEditSource implements the "config edit" command.
func runEditSource(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return errors.New("--name is required")
	}

	newName, _ := cmd.Flags().GetString("new-name")
	newURL, _ := cmd.Flags().GetString("url")
	newType, _ := cmd.Flags().GetString("type")
	newPriority, _ := cmd.Flags().GetInt("priority")
	prioritySet := cmd.Flags().Changed("priority")

	path, err := resolveConfigPath()
	if err != nil {
		return err
	}

	doc, err := loadRawYAML(path)
	if err != nil {
		return err
	}

	seq := sourcesNode(doc)
	if seq == nil {
		return errors.New("config file has no 'sources' key")
	}

	found := false
	for _, child := range seq.Content {
		if getScalarField(child, "name") != name {
			continue
		}
		found = true
		if newName != "" {
			setScalarField(child, "name", newName)
		}
		if newURL != "" {
			setScalarField(child, "url", newURL)
		}
		if newType != "" {
			setScalarField(child, "type", newType)
		}
		if prioritySet {
			setScalarField(child, "priority", strconv.Itoa(newPriority))
		}
		break
	}
	if !found {
		return fmt.Errorf("source %q not found", name)
	}

	if err := saveRawYAML(path, doc); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Updated source %q in %s\n", name, path)
	return nil
}

func init() {
	// Global config path flag on the parent command.
	configCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to config file (default: ./config.yaml or ./configs/config.yaml)")

	// add flags
	addSourceCmd.Flags().String("name", "", "Source name (required)")
	addSourceCmd.Flags().String("url", "", "RSS feed URL (required)")
	addSourceCmd.Flags().String("type", "rss", "Feed type (required, e.g. rss)")
	addSourceCmd.Flags().Int("priority", 1, "Priority (1 = highest)")

	// delete flags
	deleteSourceCmd.Flags().String("name", "", "Name of source to delete (required)")

	// edit flags
	editSourceCmd.Flags().String("name", "", "Name of source to edit (required)")
	editSourceCmd.Flags().String("new-name", "", "New name for the source")
	editSourceCmd.Flags().String("url", "", "New RSS feed URL")
	editSourceCmd.Flags().String("type", "", "New feed type")
	editSourceCmd.Flags().Int("priority", 0, "New priority")

	configCmd.AddCommand(listSourcesCmd, addSourceCmd, deleteSourceCmd, editSourceCmd)
	rootCmd.AddCommand(configCmd)
}

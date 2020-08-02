package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jaeles-project/jaeles/core"
	"github.com/jaeles-project/jaeles/database"
	"github.com/jaeles-project/jaeles/libs"
	"github.com/jaeles-project/jaeles/utils"
	"github.com/spf13/cobra"
)

func init() {
	var configCmd = &cobra.Command{
		Use:   "config",
		Short: "Configuration CLI",
		Long:  libs.Banner(),
		RunE:  runConfig,
	}
	configCmd.Flags().Bool("clean", false, "Clean old record")
	configCmd.Flags().StringP("action", "a", "", "Action")
	// used for cred action
	configCmd.Flags().String("user", "", "Username")
	configCmd.Flags().String("pass", "", "Password")
	configCmd.Flags().Bool("hh", false, "More helper")
	configCmd.Flags().Bool("mics", true, "Skip import mics signatures")
	configCmd.Flags().Bool("poll", false, "Polling all record in OOB config")
	// used for update action
	configCmd.Flags().String("secret", "", "Secret of Burp Collab")
	configCmd.Flags().String("collab", "", "List of Burp Collab File")
	configCmd.Flags().String("repo", "", "Signature Repo")
	configCmd.Flags().StringVarP(&options.Server.Key, "key", "K", "", "Private Key to pull repo")
	configCmd.SetHelpFunc(configHelp)
	RootCmd.AddCommand(configCmd)

}

func runConfig(cmd *cobra.Command, _ []string) error {
	// print more help
	helps, _ := cmd.Flags().GetBool("hh")
	mics, _ := cmd.Flags().GetBool("mics")
	if helps == true {
		HelpMessage()
		os.Exit(1)
	}
	// turn on verbose by default
	options.Verbose = true

	polling, _ := cmd.Flags().GetBool("poll")
	// polling all oob
	if polling == true {
		secret, _ := cmd.Flags().GetString("secret")
		collabFile, _ := cmd.Flags().GetString("collab")
		collabs := utils.ReadingLines(collabFile)
		for _, collab := range collabs {
			database.ImportCollab(secret, collab)
		}
	}

	action, _ := cmd.Flags().GetString("action")
	switch action {
	case "update":
		// in case we want to in private repo
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("pass")
		options.Server.Username = username
		options.Server.Password = password
		core.UpdatePlugins(options)
		repo, _ := cmd.Flags().GetString("repo")
		core.UpdateSignature(options, repo)
		reloadSignature(path.Join(options.RootFolder, "base-signatures"), mics)
		break
	case "clear":
		database.CleanScans()
		database.CleanSigns()
		database.CleanRecords()
		break
	case "clean":
		os.RemoveAll(options.RootFolder)
		break
	case "cred":
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("pass")
		database.CreateUser(username, password)
		utils.GoodF("Create new credentials %v:%v \n", username, password)
		break
	case "oob":
		secret, _ := cmd.Flags().GetString("secret")
		collabFile, _ := cmd.Flags().GetString("collab")
		collabs := utils.ReadingLines(collabFile)
		for _, collab := range collabs {
			database.ImportCollab(secret, collab)
		}
		break
	case "init":
		reloadSignature(options.SignFolder, mics)
		break
	case "reload":
		os.RemoveAll(path.Join(options.RootFolder, "base-signatures"))
		reloadSignature(options.SignFolder, mics)
		break
	case "add":
		addSignature(options.SignFolder)
		break
	default:
		HelpMessage()
	}
	return nil
}

// addSignature add active signatures from a folder
func addSignature(signFolder string) {
	signFolder = utils.NormalizePath(signFolder)
	if !utils.FolderExists(signFolder) {
		utils.ErrorF("Signature folder not found: %v", signFolder)
		return
	}
	allSigns := utils.GetFileNames(signFolder, ".yaml")
	if allSigns != nil {
		utils.InforF("Add Signature from: %v", signFolder)
		for _, signFile := range allSigns {
			database.ImportSign(signFile)
		}
	}
}

// reloadSignature signature
func reloadSignature(signFolder string, skipMics bool) {
	signFolder = utils.NormalizePath(signFolder)
	if !utils.FolderExists(signFolder) {
		utils.ErrorF("Signature folder not found: %v", signFolder)
		return
	}
	utils.GoodF("Reload signature in: %v", signFolder)
	database.CleanSigns()
	SignFolder, _ := filepath.Abs(path.Join(options.RootFolder, "base-signatures"))
	if signFolder != "" && utils.FolderExists(signFolder) {
		SignFolder = signFolder
	}
	allSigns := utils.GetFileNames(SignFolder, ".yaml")
	if allSigns != nil {
		utils.InforF("Load Signature from: %v", SignFolder)
		for _, signFile := range allSigns {
			if skipMics {
				if strings.Contains(signFile, "/mics/") {
					utils.DebugF("Skip sign: %v", signFile)
					continue
				}

				if strings.Contains(signFile, "/exper/") {
					utils.DebugF("Skip sign: %v", signFile)
					continue
				}
			}
			database.ImportSign(signFile)
		}
	}

	signPath := path.Join(options.RootFolder, "base-signatures")
	passivePath := path.Join(signPath, "passives")
	resourcesPath := path.Join(signPath, "resources")
	thirdpartyPath := path.Join(signPath, "thirdparty")

	// copy it to base signature folder
	if !utils.FolderExists(signPath) {
		utils.CopyDir(signFolder, signPath)
	}

	// move passive signatures to default passive
	if utils.FolderExists(passivePath) {
		utils.MoveFolder(passivePath, options.PassiveFolder)
	}
	if utils.FolderExists(resourcesPath) {
		utils.MoveFolder(resourcesPath, options.ResourcesFolder)
	}
	if utils.FolderExists(thirdpartyPath) {
		utils.MoveFolder(thirdpartyPath, options.ThirdPartyFolder)
	}

}

func configHelp(_ *cobra.Command, _ []string) {
	fmt.Println(libs.Banner())
	HelpMessage()
}

func rootHelp(cmd *cobra.Command, _ []string) {
	fmt.Println(libs.Banner())
	helps, _ := cmd.Flags().GetBool("hh")
	if helps {
		fmt.Println(cmd.UsageString())
		return
	}
	RootMessage()
}

// RootMessage print help message
func RootMessage() {
	h := "\nUsage:\n jaeles scan|server|config [options]\n"
	h += " jaeles scan|server|config|report -h -- Show usage message\n"
	h += "\nSubcommands:\n"
	h += "  jaeles scan   --  Scan list of URLs based on selected signatures\n"
	h += "  jaeles server --  Start API server\n"
	h += "  jaeles config --  Configuration CLI \n"
	h += "  jaeles report --  Generate HTML report based on scanned output \n"
	h += `
Core Flags:
  -c, --concurrency int         Set the concurrency level (default 20)
  -o, --output string           Output folder name (default "out")
  -s, --signs strings           Signature selector (Multiple -s flags are accepted)
  -x, --exclude strings         Exclude Signature selector (Multiple -x flags are accepted)
  -L, --level int               Filter signatures by level (default 1)
  -G, --passive                 Turn on passive detections
  -p, --params strings          Custom params -p='foo=bar' (Multiple -p flags are accepted)
  -H, --headers strings         Custom headers (e.g: -H 'Referer: {{.BaseURL}}') (Multiple -H flags are accepted)

Mics Flags:
      --proxy string            proxy
      --timeout int             HTTP timeout (default 20)
      --debug                   Debug
  -v, --verbose                 Verbose
      --no-db                   Disable Database
  -S, --selectorFile string     Signature selector from file
  -J, --format-input            Enable special input format (default is false)
  -f, --found string            Run host OS command when vulnerable found
  -O, --summaryOutput string    Summary output file (default is "jaeles-summary.txt")
      --passiveOutput string    Passive output folder (default is "passive-out")
      --passiveSummary string   Passive Summary file
      --sp string               Selector for passive detections (default "*")
      --single string           Forced running in single mode
      --sverbose bool           Store verbose info in summary file
  -N  --no-output bool          Disable store output
      --json bool               Store output as JSON format
  -q, --quiet                   Enable Quiet Output
  -Q, --quietFormat string      Format for quiet output (default "{{.VulnURL}}")
  -R, --report string           HTML report file name
      --title string            HTML report title
      --html string             Enable generate HTML reports after the scan done 
      --hh string               Full help message
      --lc                      Shortcut for '--proxy http://127.0.0.1:8080'
      --ba                      Shortcut for -p 'BaseURL=[[.Raw]]' or -p 'root=[[.Raw]]'
`
	h += "\n\nExamples Commands:\n"
	h += "  jaeles scan -s <signature> -u <url>\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls> -L <level-of-signatures>\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls>\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls> -p 'dest=xxx.burpcollaborator.net'\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls> -f 'noti_slack \"{{.vulnInfo}}\"'\n"
	h += "  jaeles scan -v -c 50 -s <signature> -U list_target.txt -o /tmp/output\n"
	h += "  jaeles scan -s <signature> -s <another-selector> -u http://example.com\n"
	h += "  echo '{\"BaseURL\":\"https://example.com/sub/\"}' | jaeles scan -s sign.yaml -J \n"
	h += "  jaeles scan -G -s <signature> -s <another-selector> -x <exclude-selector> -u http://example.com\n"
	h += "  cat list_target.txt | jaeles scan -c 100 -s <signature>\n"

	h += "\nOthers Commands:\n"
	h += "  jaeles server -s '/tmp/custom-signature/sensitive/.*' -L 2\n"
	h += "  jaeles server --host 0.0.0.0 --port 5000 -s '/tmp/custom-signature/sensitive/.*' -L 2\n"
	h += "  jaeles config -a reload --signDir /tmp/standard-signatures/\n"
	h += "  jaeles config -a add -B /tmp/custom-active-signatures/\n"
	h += "  jaeles config -a update --repo https://github.com/jaeles-project/jaeles-signatures\n"
	h += "  jaeles report -o /tmp/scanned/out\n"
	h += "  jaeles report -o /tmp/scanned/out --title 'Passive Report'\n"
	h += "  jaeles report -o /tmp/scanned/out --title 'Verbose Report' --sverbose\n"
	h += "\nOfficial Documentation can be found here: https://jaeles-project.github.io/\n"
	fmt.Println(h)
}

// HelpMessage print help message
func HelpMessage() {
	h := "\nConfig Command example:\n\n"
	h += "  jaeles config -a init\n\n"
	h += "  jaeles config -a update --repo http://github.com/jaeles-project/another-signatures --user admin --pass admin\n"
	h += "  jaeles config -a update --repo git@github.com/jaeles-project/another-signatures -K your_private_key\n"
	h += "  jaeles config -a clean\n"
	h += "  jaeles config -a reload\n"
	h += "  jaeles config -a reload --signDir /tmp/standard-signatures/\n"
	h += "  jaeles config -a add --signDir /tmp/standard-signatures/\n"
	h += "  jaeles config -a cred --user sample --pass not123456\n\n"
	fmt.Println(h)
}

func ScanHelp(cmd *cobra.Command, _ []string) {
	fmt.Println(libs.Banner())
	fmt.Println(cmd.UsageString())
	ScanMessage()
}

// ScanMessage print help message
func ScanMessage() {
	h := "\nScan Usage example:\n"
	h += "  jaeles scan -s <signature> -u <url>\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls> -L <level-of-signatures>\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls>\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls> -p 'dest=xxx.burpcollaborator.net'\n"
	h += "  jaeles scan -c 50 -s <signature> -U <list_urls> -f 'noti_slack \"{{.vulnInfo}}\"'\n"
	h += "  jaeles scan -v -c 50 -s <signature> -U list_target.txt -o /tmp/output\n"
	h += "  jaeles scan -s <signature> -s <another-selector> -u http://example.com\n"
	h += "  echo '{\"BaseURL\":\"https://example.com/sub/\"}' | jaeles scan -s sign.yaml -J \n"
	h += "  jaeles scan -G -s <signature> -s <another-selector> -x <exclude-selector> -u http://example.com\n"
	h += "  cat list_target.txt | jaeles scan -c 100 -s <signature>\n"

	h += "\n\nExamples:\n"
	h += "  jaeles scan -s 'jira' -s 'ruby' -u target.com\n"
	h += "  jaeles scan -c 50 -s 'java' -x 'tomcat' -U list_of_urls.txt\n"
	h += "  jaeles scan -G -c 50 -s '/tmp/custom-signature/.*' -U list_of_urls.txt\n"
	h += "  jaeles scan -v -s '~/my-signatures/products/wordpress/.*' -u 'https://wp.example.com' -p 'root=[[.URL]]'\n"
	h += "  cat urls.txt | grep 'interesting' | jaeles scan -L 5 -c 50 -s 'fuzz/.*' -U list_of_urls.txt --proxy http://127.0.0.1:8080\n"
	h += "\n"
	fmt.Println(h)
}

// ServerHelp report help message
func ServerHelp(cmd *cobra.Command, _ []string) {
	fmt.Println(libs.Banner())
	fmt.Println(cmd.UsageString())
}

// ReportHelp report help message
func ReportHelp(cmd *cobra.Command, _ []string) {
	fmt.Println(libs.Banner())
	fmt.Println(cmd.UsageString())
}

func CleanOutput() {
	// clean output
	if utils.DirLength(options.Output) == 0 {
		os.RemoveAll(options.Output)
	}
	if utils.DirLength(options.PassiveFolder) == 0 {
		os.RemoveAll(options.PassiveFolder)
	}

	// unique vulnSummary
	// Sort sort content of a file
	data := utils.ReadingFileUnique(options.SummaryVuln)
	if len(data) == 0 {
		return
	}
	sort.Strings(data)
	content := strings.Join(data, "\n")
	// remove blank line
	content = regexp.MustCompile(`[\t\r\n]+`).ReplaceAllString(strings.TrimSpace(content), "\n")
	utils.WriteToFile(options.SummaryVuln, content)
}

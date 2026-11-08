// skillcheck 命令校验 skills/ 下的技能同时符合 Agent Skills（Claude）与 Codex 的格式，
// 并维护每个技能的适用 AlonaBot 版本与最后更新日期。
//
// 用法：
//
//	go run ./tools/skillcheck check [--git]       # 校验全部技能；--git 另核对更新日期不早于最后一次修改该技能的提交
//	go run ./tools/skillcheck stamp [技能名...]    # 把指定技能的日期改为今天、版本改为 VERSION
//	go run ./tools/skillcheck stamp --all         # 修改 VERSION 后更新全部技能
//	go run ./tools/skillcheck stamp --staged      # pre-commit 使用：只处理暂存区里有改动的技能并重新暂存
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const (
	skillsDir   = "skills"
	versionFile = "VERSION"
	marketFile  = ".claude-plugin/marketplace.json"
	readmeFile  = "README.md"
	dateLayout  = "2006-01-02"
	maxBodyLine = 500
)

var (
	namePattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.]+)?$`)
	// stampPattern 匹配 Markdown 中给读者看的版本与日期行。
	stampPattern = regexp.MustCompile(`(?m)^> AlonaBot (\S+) · 最后更新 (\S+)$`)
	linkPattern  = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	// readmeRow 匹配 README 技能表的一行，捕获技能名与最后更新日期。
	readmeRow = regexp.MustCompile(`(?m)^(\| \[([a-z0-9-]+)\]\(skills/[a-z0-9-]+/SKILL\.md\) \|.*\| )([0-9]{4}-[0-9]{2}-[0-9]{2}) \|$`)
	// 以下两项匹配 SKILL.md frontmatter 中 metadata 的对应行，stamp 只改这两行。
	metaVersionLine = regexp.MustCompile(`(?m)^(\s+alonabot-version:\s*).*$`)
	metaUpdatedLine = regexp.MustCompile(`(?m)^(\s+updated:\s*).*$`)
)

// allowedKeys 是 Agent Skills 规范定义的全部顶层 frontmatter 键；其他键在部分客户端会被拒绝。
var allowedKeys = []string{"name", "description", "license", "compatibility", "metadata", "allowed-tools"}

// reservedWords 是 Claude 禁止出现在技能名中的词。
var reservedWords = []string{"anthropic", "claude"}

// frontmatter 是 SKILL.md 头部的 YAML 元数据。
type frontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
}

// openAIConfig 是 Codex 读取的 agents/openai.yaml。
type openAIConfig struct {
	Interface struct {
		DisplayName      string `yaml:"display_name"`
		ShortDescription string `yaml:"short_description"`
		DefaultPrompt    string `yaml:"default_prompt"`
	} `yaml:"interface"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		useGit := fs.Bool("git", false, "also require metadata.updated to be no earlier than the last commit touching the skill")
		_ = fs.Parse(os.Args[2:])
		err = runCheck(*useGit)
	case "stamp":
		fs := flag.NewFlagSet("stamp", flag.ExitOnError)
		staged := fs.Bool("staged", false, "stamp skills with staged changes and re-stage them")
		all := fs.Bool("all", false, "stamp every skill, e.g. after changing VERSION")
		_ = fs.Parse(os.Args[2:])
		names := fs.Args()
		if *all {
			if names, err = listSkills(); err != nil {
				break
			}
		}
		err = runStamp(*staged, names)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: skillcheck check [--git] | stamp [--all | --staged | skill...]")
	os.Exit(2)
}

// readVersion 读取仓库根目录 VERSION 中的 AlonaBot 版本。
func readVersion() (string, error) {
	b, err := os.ReadFile(versionFile)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", versionFile, err)
	}
	v := strings.TrimSpace(string(b))
	if !versionPattern.MatchString(v) {
		return "", fmt.Errorf("%s: %q is not a version such as 0.1-beta", versionFile, v)
	}
	return v, nil
}

// listSkills 返回 skills/ 下按名称排序的技能目录名。
func listSkills() ([]string, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", skillsDir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// splitFrontmatter 把 SKILL.md 拆成 YAML 头与正文。
func splitFrontmatter(content []byte) (head, body []byte, err error) {
	rest, ok := bytes.CutPrefix(content, []byte("---\n"))
	if !ok {
		return nil, nil, errors.New("SKILL.md must start with a --- frontmatter line")
	}
	head, body, ok = bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return nil, nil, errors.New("SKILL.md frontmatter has no closing --- line")
	}
	return head, body, nil
}

func runCheck(useGit bool) error {
	version, err := readVersion()
	if err != nil {
		return err
	}
	names, err := listSkills()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%s: no skills found", skillsDir)
	}
	var problems []string
	for _, name := range names {
		for _, p := range checkSkill(name, version, useGit) {
			problems = append(problems, filepath.Join(skillsDir, name)+": "+p)
		}
	}
	problems = append(problems, checkMarketplace(names, version)...)
	problems = append(problems, checkReadme(names)...)
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	fmt.Printf("ok: %d skills, AlonaBot %s\n", len(names), version)
	return nil
}

// checkMarketplace 核对 Claude Code 插件市场清单的版本与技能列表。
func checkMarketplace(names []string, version string) []string {
	raw, err := os.ReadFile(marketFile)
	if err != nil {
		return []string{marketFile + ": " + err.Error()}
	}
	var m struct {
		Metadata struct {
			Version string `json:"version"`
		} `json:"metadata"`
		Plugins []struct {
			Skills []string `json:"skills"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return []string{marketFile + ": " + err.Error()}
	}
	var problems []string
	if m.Metadata.Version != version {
		problems = append(problems, fmt.Sprintf("%s: metadata.version %q must equal %s (%s)", marketFile, m.Metadata.Version, versionFile, version))
	}
	var listed []string
	for _, p := range m.Plugins {
		for _, s := range p.Skills {
			listed = append(listed, strings.TrimPrefix(s, "./"+skillsDir+"/"))
		}
	}
	slices.Sort(listed)
	if !slices.Equal(listed, names) {
		problems = append(problems, fmt.Sprintf("%s: plugins[].skills lists %v, but %s/ has %v", marketFile, listed, skillsDir, names))
	}
	return problems
}

// checkReadme 核对 README 技能表列出全部技能，且日期与各自的 metadata.updated 一致。
func checkReadme(names []string) []string {
	raw, err := os.ReadFile(readmeFile)
	if err != nil {
		return []string{readmeFile + ": " + err.Error()}
	}
	rows := map[string]string{}
	for _, m := range readmeRow.FindAllSubmatch(raw, -1) {
		rows[string(m[2])] = string(m[3])
	}
	var problems []string
	for _, name := range names {
		got, ok := rows[name]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: skills table has no row for %s", readmeFile, name))
			continue
		}
		if want := skillDate(name); got != want {
			problems = append(problems, fmt.Sprintf("%s: %s row says %s but metadata.updated is %s; run skillcheck stamp %s", readmeFile, name, got, want, name))
		}
	}
	return problems
}

// skillDate 返回技能 frontmatter 中的 metadata.updated；读不到时返回空串，由 checkSkill 报告原因。
func skillDate(name string) string {
	content, err := os.ReadFile(filepath.Join(skillsDir, name, "SKILL.md"))
	if err != nil {
		return ""
	}
	head, _, err := splitFrontmatter(content)
	if err != nil {
		return ""
	}
	var fm frontmatter
	if yaml.Unmarshal(head, &fm) != nil {
		return ""
	}
	return fm.Metadata["updated"]
}

// stampReadme 把 README 技能表中这些技能的日期改为 today，返回是否改动。
func stampReadme(names []string, today string) (bool, error) {
	old, err := os.ReadFile(readmeFile)
	if err != nil {
		return false, err
	}
	updated := readmeRow.ReplaceAllFunc(old, func(row []byte) []byte {
		m := readmeRow.FindSubmatch(row)
		if !slices.Contains(names, string(m[2])) {
			return row
		}
		return slices.Concat(m[1], []byte(today+" |"))
	})
	if bytes.Equal(old, updated) {
		return false, nil
	}
	return true, os.WriteFile(readmeFile, updated, 0o644)
}

// checkSkill 返回一个技能的全部问题。
func checkSkill(name, version string, useGit bool) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	dir := filepath.Join(skillsDir, name)

	// 1. 解析 SKILL.md 与 frontmatter。
	content, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return []string{"missing SKILL.md"}
	}
	head, body, err := splitFrontmatter(content)
	if err != nil {
		return []string{err.Error()}
	}
	var keys map[string]yaml.Node
	if err := yaml.Unmarshal(head, &keys); err != nil {
		return []string{"frontmatter: " + err.Error()}
	}
	for k := range keys {
		if !slices.Contains(allowedKeys, k) {
			add("frontmatter: key %q is not in the Agent Skills spec; put custom fields under metadata", k)
		}
	}
	var fm frontmatter
	if err := yaml.Unmarshal(head, &fm); err != nil {
		return append(problems, "frontmatter: "+err.Error()+" (metadata values must be strings)")
	}

	// 2. name 与 description 按两家规则中较严的一方检查。
	switch {
	case fm.Name != name:
		add("name %q must equal the directory name", fm.Name)
	case len(fm.Name) > 64 || !namePattern.MatchString(fm.Name):
		add("name must be 1-64 lowercase letters, digits and single hyphens")
	}
	for _, w := range reservedWords {
		if strings.Contains(fm.Name, w) {
			add("name must not contain the reserved word %q", w)
		}
	}
	if n := utf8.RuneCountInString(fm.Description); n == 0 || n > 1024 {
		add("description must be 1-1024 characters, got %d", n)
	}
	if strings.ContainsAny(fm.Description, "<>") {
		add("description must not contain < or >")
	}
	if utf8.RuneCountInString(fm.Compatibility) > 500 {
		add("compatibility must be at most 500 characters")
	}

	// 3. 版本与日期：metadata 是唯一来源，正文与参考文件中的可见行必须一致。
	gotVersion, gotDate := fm.Metadata["alonabot-version"], fm.Metadata["updated"]
	if gotVersion != version {
		add("metadata.alonabot-version %q must equal %s (%s); run skillcheck stamp %s", gotVersion, versionFile, version, name)
	}
	date, dateErr := time.Parse(dateLayout, gotDate)
	if dateErr != nil {
		add("metadata.updated %q must be a YYYY-MM-DD date", gotDate)
	} else if date.After(time.Now().AddDate(0, 0, 1)) {
		add("metadata.updated %s is in the future", gotDate)
	}
	if m := stampPattern.FindSubmatch(body); m == nil {
		add("SKILL.md body needs a line \"> AlonaBot %s · 最后更新 %s\" under the title", gotVersion, gotDate)
	}
	if lines := bytes.Count(body, []byte("\n")); lines > maxBodyLine {
		add("SKILL.md body has %d lines; keep it under %d and move details to references/", lines, maxBodyLine)
	}

	// 4. 目录内全部 Markdown：可见行一致、相对链接可达。
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		md, err := os.ReadFile(path)
		if err != nil {
			add("%s: %v", path, err)
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		for _, m := range stampPattern.FindAllSubmatch(md, -1) {
			if string(m[1]) != gotVersion || string(m[2]) != gotDate {
				add("%s: visible line says %s · %s but metadata says %s · %s", rel, m[1], m[2], gotVersion, gotDate)
			}
		}
		for _, m := range linkPattern.FindAllSubmatch(md, -1) {
			target := string(m[1])
			if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), target)); err != nil {
				add("%s: broken link %s", rel, m[1])
			}
		}
		return nil
	})

	// 5. Codex 的 agents/openai.yaml。
	if raw, err := os.ReadFile(filepath.Join(dir, "agents", "openai.yaml")); err != nil {
		add("missing agents/openai.yaml for Codex")
	} else {
		var oc openAIConfig
		if err := yaml.Unmarshal(raw, &oc); err != nil {
			add("agents/openai.yaml: %v", err)
		}
		if oc.Interface.DisplayName == "" || oc.Interface.ShortDescription == "" {
			add("agents/openai.yaml: interface.display_name and interface.short_description are required")
		}
		if !strings.Contains(oc.Interface.DefaultPrompt, "$"+name) {
			add("agents/openai.yaml: interface.default_prompt must mention $%s", name)
		}
	}

	// 6. 可选：日期不早于最后一次修改该技能的提交。
	if useGit && dateErr == nil {
		out, gitErr := exec.Command("git", "log", "-1", "--format=%as", "--", dir).Output()
		if gitErr != nil {
			add("git log: %v", gitErr)
		} else if last := strings.TrimSpace(string(out)); last != "" && gotDate < last {
			add("metadata.updated %s is older than the last commit touching this skill (%s); run skillcheck stamp %s", gotDate, last, name)
		}
	}
	return problems
}

func runStamp(staged bool, names []string) error {
	version, err := readVersion()
	if err != nil {
		return err
	}
	if staged {
		if names, err = stagedSkills(); err != nil {
			return err
		}
		if err := ensureFullyStaged(names); err != nil {
			return err
		}
	} else if len(names) == 0 {
		return errors.New("name the skills to stamp, or use --all or --staged")
	}
	today := time.Now().Format(dateLayout)
	for _, name := range names {
		files, err := stampSkill(name, version, today)
		if err != nil {
			return err
		}
		if staged && len(files) > 0 {
			if err := restage(files); err != nil {
				return err
			}
		}
		for _, f := range files {
			fmt.Printf("stamped %s: AlonaBot %s · %s\n", f, version, today)
		}
	}
	changed, err := stampReadme(names, today)
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("stamped %s\n", readmeFile)
		if staged {
			return restage([]string{readmeFile})
		}
	}
	return nil
}

// stagedSkills 返回暂存区中有改动的技能名。
func stagedSkills() ([]string, error) {
	out, err := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--", skillsDir).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		parts := strings.Split(sc.Text(), "/")
		if len(parts) >= 3 && !slices.Contains(names, parts[1]) {
			names = append(names, parts[1])
		}
	}
	return names, sc.Err()
}

// stampSkill 更新一个技能的 metadata 与全部可见行，返回改动过的文件。
func stampSkill(name, version, today string) ([]string, error) {
	dir := filepath.Join(skillsDir, name)
	var changed []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := stampPattern.ReplaceAll(old, []byte("> AlonaBot "+version+" · 最后更新 "+today))
		if filepath.Base(path) == "SKILL.md" {
			head, body, err := splitFrontmatter(updated)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if !metaVersionLine.Match(head) || !metaUpdatedLine.Match(head) {
				return fmt.Errorf("%s: metadata needs alonabot-version and updated lines", path)
			}
			head = metaVersionLine.ReplaceAll(head, []byte(`${1}"`+version+`"`))
			head = metaUpdatedLine.ReplaceAll(head, []byte(`${1}"`+today+`"`))
			updated = slices.Concat([]byte("---\n"), head, []byte("\n---\n"), body)
		}
		if bytes.Equal(old, updated) {
			return nil
		}
		changed = append(changed, path)
		return os.WriteFile(path, updated, 0o644)
	})
	return changed, err
}

// ensureFullyStaged 在 --staged 模式下拒绝另有未暂存改动的技能，避免 git add 把它们一起提交。
func ensureFullyStaged(names []string) error {
	if len(names) == 0 {
		return nil
	}
	if out, err := exec.Command("git", "diff", "--name-only", "--", readmeFile).Output(); err != nil {
		return fmt.Errorf("git diff: %w", err)
	} else if len(bytes.TrimSpace(out)) > 0 {
		return fmt.Errorf("%s has unstaged changes; stage them first so the skills table can be re-staged", readmeFile)
	}
	for _, name := range names {
		out, err := exec.Command("git", "diff", "--name-only", "--", filepath.Join(skillsDir, name)).Output()
		if err != nil {
			return fmt.Errorf("git diff: %w", err)
		}
		if dirty := strings.TrimSpace(string(out)); dirty != "" {
			return fmt.Errorf("%s has unstaged changes:\n%s\nstage them first, or run: go run ./tools/skillcheck stamp %s", filepath.Join(skillsDir, name), dirty, name)
		}
	}
	return nil
}

// restage 重新暂存 stamp 改过的文件。
func restage(files []string) error {
	if out, err := exec.Command("git", append([]string{"add", "--"}, files...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %v: %s", err, out)
	}
	return nil
}

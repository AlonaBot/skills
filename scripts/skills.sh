#!/bin/sh
# skills.sh 校验 skills/ 下的技能同时符合 Agent Skills（Claude）与 Codex 的格式，
# 并维护每个技能的适用 AlonaBot 版本与最后更新日期。只用 POSIX sh、awk、grep 与 git，不需要编译或安装任何语言。
#
# 用法：
#   sh scripts/skills.sh check [--git]          校验全部技能；--git 另核对更新日期不早于最后一次修改该技能的提交
#   sh scripts/skills.sh stamp 技能名...         把指定技能的日期改为今天、版本改为 VERSION
#   sh scripts/skills.sh stamp --all            修改 VERSION 后更新全部技能
#   sh scripts/skills.sh stamp --staged         pre-commit 使用：只处理暂存区里有改动的技能并重新暂存
#
# 为了不依赖 YAML 解析器，frontmatter 限定为简单写法：顶层键各占一行且值写在同一行；
# metadata 下每项缩进两个空格，值用双引号。check 会拒绝其他写法。

set -eu

SKILLS_DIR=skills
VERSION_FILE=VERSION
MARKET_FILE=.claude-plugin/marketplace.json
README_FILE=README.md
MAX_BODY_LINES=500
ALLOWED_KEYS=" name description license compatibility metadata allowed-tools "

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

problems=0
fail() {
	printf '%s\n' "$*" >&2
	problems=$((problems + 1))
}

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

# chars 输出 UTF-8 字符串的字符数（去掉续字节后计数，不依赖 locale）。
chars() {
	printf '%s' "$1" | LC_ALL=C tr -d '\200-\277' | wc -c | tr -d ' '
}

# unquote 去掉值两端的一对双引号或单引号。
unquote() {
	printf '%s' "$1" | sed -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'\$/\1/"
}

# less_than 在字符串 $1 按字典序小于 $2 时成功；YYYY-MM-DD 日期可以直接这样比较。
less_than() {
	awk -v a="$1" -v b="$2" 'BEGIN { exit !(a < b) }'
}

read_version() {
	[ -f "$VERSION_FILE" ] || die "$VERSION_FILE: missing"
	version=$(tr -d ' \r\n' <"$VERSION_FILE")
	printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.]+)?$' ||
		die "$VERSION_FILE: \"$version\" is not a version such as 0.1-beta"
}

list_skills() {
	for d in "$SKILLS_DIR"/*/; do
		[ -d "$d" ] && basename "$d"
	done
	return 0
}

# frontmatter 输出 SKILL.md 两条 --- 之间的内容；格式不对时什么也不输出并返回 1。
frontmatter() {
	awk 'NR == 1 { if ($0 != "---") exit 1; next } $0 == "---" { closed = 1; exit 0 } { print } END { if (!closed) exit 1 }' "$1"
}

# body 输出 frontmatter 之后的正文。
body() {
	awk 'NR == 1 { next } !done && $0 == "---" { done = 1; next } done { print }' "$1"
}

# top_value 输出 frontmatter 中顶层键 $2 的值。
top_value() {
	printf '%s\n' "$1" | awk -v k="$2" 'index($0, k ":") == 1 { v = substr($0, length(k) + 2); sub(/^[ \t]+/, "", v); print v; exit }'
}

# meta_value 输出 metadata 下键 $2 的值。
meta_value() {
	printf '%s\n' "$1" | awk -v k="$2" '
		/^metadata:/ { in_meta = 1; next }
		/^[^ ]/ { in_meta = 0 }
		in_meta && index($0, "  " k ":") == 1 { v = substr($0, length(k) + 4); sub(/^[ \t]+/, "", v); print v; exit }'
}

check_skill() {
	name=$1
	dir=$SKILLS_DIR/$name
	at="$dir:"
	skill=$dir/SKILL.md

	# 1. 解析 SKILL.md 与 frontmatter。
	if [ ! -f "$skill" ]; then
		fail "$at missing SKILL.md"
		return 0
	fi
	if ! head=$(frontmatter "$skill") || [ -z "$head" ]; then
		fail "$at SKILL.md must start with a --- frontmatter block closed by another --- line"
		return 0
	fi
	bad=$(printf '%s\n' "$head" | awk '
		/^[ \t]*$/ { next }
		/^metadata:[ \t]*$/ { in_meta = 1; next }
		in_meta && /^  [a-z0-9][a-z0-9-]*: "[^"]*"[ \t]*$/ { next }
		/^[a-z][a-z-]*: [^ ]/ { in_meta = 0; next }
		{ print "frontmatter line not in the supported simple form: " $0 }')
	[ -z "$bad" ] || fail "$at $bad (metadata values must be double-quoted strings)"
	for key in $(printf '%s\n' "$head" | sed -n 's/^\([a-z][a-z-]*\):.*/\1/p'); do
		case "$ALLOWED_KEYS" in
		*" $key "*) ;;
		*) fail "$at frontmatter key \"$key\" is not in the Agent Skills spec; put custom fields under metadata" ;;
		esac
	done

	# 2. name 与 description 按两家规则中较严的一方检查。
	fm_name=$(unquote "$(top_value "$head" name)")
	if [ "$fm_name" != "$name" ]; then
		fail "$at name \"$fm_name\" must equal the directory name"
	elif [ "$(chars "$fm_name")" -gt 64 ] || ! printf '%s\n' "$fm_name" | grep -Eq '^[a-z0-9]+(-[a-z0-9]+)*$'; then
		fail "$at name must be 1-64 lowercase letters, digits and single hyphens"
	fi
	for word in anthropic claude; do
		case "$fm_name" in *"$word"*) fail "$at name must not contain the reserved word \"$word\"" ;; esac
	done
	desc=$(unquote "$(top_value "$head" description)")
	n=$(chars "$desc")
	if [ "$n" -eq 0 ] || [ "$n" -gt 1024 ]; then
		fail "$at description must be 1-1024 characters, got $n"
	fi
	case "$desc" in *"<"* | *">"*) fail "$at description must not contain < or >" ;; esac
	compat=$(unquote "$(top_value "$head" compatibility)")
	[ "$(chars "$compat")" -le 500 ] || fail "$at compatibility must be at most 500 characters"

	# 3. 版本与日期：metadata 是唯一来源，正文与参考文件中的可见行必须一致。
	got_version=$(unquote "$(meta_value "$head" alonabot-version)")
	got_date=$(unquote "$(meta_value "$head" updated)")
	[ "$got_version" = "$version" ] ||
		fail "$at metadata.alonabot-version \"$got_version\" must equal $VERSION_FILE ($version); run: sh scripts/skills.sh stamp $name"
	date_ok=0
	if ! printf '%s\n' "$got_date" | grep -Eq '^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'; then
		fail "$at metadata.updated \"$got_date\" must be a YYYY-MM-DD date"
	elif less_than "$latest_today" "$got_date"; then
		fail "$at metadata.updated $got_date is in the future"
	else
		date_ok=1
	fi
	visible="> AlonaBot $got_version · 最后更新 $got_date"
	body "$skill" | grep -Fxq "$visible" ||
		fail "$at SKILL.md body needs the line \"$visible\" under the title"
	lines=$(body "$skill" | wc -l | tr -d ' ')
	[ "$lines" -le "$MAX_BODY_LINES" ] ||
		fail "$at SKILL.md body has $lines lines; keep it under $MAX_BODY_LINES and move details to references/"

	# 4. 目录内全部 Markdown：可见行一致、相对链接可达。
	for md in $(find "$dir" -type f -name '*.md' | sort); do
		rel=${md#"$dir"/}
		mismatch=$(grep -E '^> AlonaBot [^ ]+ · 最后更新 [^ ]+$' "$md" | grep -Fxv "$visible" | head -n 1 || true)
		[ -z "$mismatch" ] || fail "$at $rel: visible line \"$mismatch\" does not match metadata ($got_version · $got_date)"
		md_dir=$(dirname "$md")
		# 跳过代码块与行内代码，避免把 Go 泛型调用识别为 Markdown 链接。
		links=$(awk '
			/^```/ { in_code = !in_code; next }
			!in_code { gsub(/`[^`]*`/, ""); print }
		' "$md" | grep -oE '\]\([^) ]+\)' | sed -e 's/^](//' -e 's/)$//' || true)
		for target in $links; do
			case "$target" in *://* | \#* | mailto:*) continue ;; esac
			path=${target%%#*}
			[ -e "$md_dir/$path" ] || fail "$at $rel: broken link $target"
		done
	done

	# 5. Codex 的 agents/openai.yaml。
	oa=$dir/agents/openai.yaml
	if [ ! -f "$oa" ]; then
		fail "$at missing agents/openai.yaml for Codex"
	else
		grep -Eq '^interface:[ \t]*$' "$oa" || fail "$at agents/openai.yaml: missing interface:"
		grep -Eq '^  display_name: "[^"]+"' "$oa" || fail "$at agents/openai.yaml: interface.display_name is required"
		grep -Eq '^  short_description: "[^"]+"' "$oa" || fail "$at agents/openai.yaml: interface.short_description is required"
		grep -E '^  default_prompt: ' "$oa" | grep -Fq "\$$name" ||
			fail "$at agents/openai.yaml: interface.default_prompt must mention \$$name"
	fi

	# 6. 可选：日期不早于最后一次修改该技能的提交。
	if [ "$use_git" = 1 ] && [ "$date_ok" = 1 ]; then
		last=$(git log -1 --format=%as -- "$dir")
		if [ -n "$last" ] && less_than "$got_date" "$last"; then
			fail "$at metadata.updated $got_date is older than the last commit touching this skill ($last); run: sh scripts/skills.sh stamp $name"
		fi
	fi
	return 0
}

# check_marketplace 核对 Claude Code 插件市场清单的版本与技能列表。
check_marketplace() {
	if [ ! -f "$MARKET_FILE" ]; then
		fail "$MARKET_FILE: missing"
		return 0
	fi
	grep -Eq "^    \"version\": \"$(printf '%s' "$version" | sed 's/\./\\./g')\"" "$MARKET_FILE" ||
		fail "$MARKET_FILE: metadata.version must equal $VERSION_FILE ($version)"
	listed=$(grep -oE "\"\\./$SKILLS_DIR/[^\"]+\"" "$MARKET_FILE" | sed -e "s|^\"\\./$SKILLS_DIR/||" -e 's|"$||' | sort | tr '\n' ' ')
	have=$(list_skills | sort | tr '\n' ' ')
	[ "$listed" = "$have" ] || fail "$MARKET_FILE: plugins[].skills lists [${listed% }], but $SKILLS_DIR/ has [${have% }]"
	return 0
}

# check_readme 核对 README 技能表列出全部技能，且日期与各自的 metadata.updated 一致。
check_readme() {
	for name in $(list_skills); do
		row=$(grep -F "| [$name]($SKILLS_DIR/$name/SKILL.md) |" "$README_FILE" || true)
		if [ -z "$row" ]; then
			fail "$README_FILE: skills table has no row for $name"
			continue
		fi
		got=$(printf '%s\n' "$row" | sed -n 's/.*| \([0-9-]*\) |$/\1/p')
		want=$(unquote "$(meta_value "$(frontmatter "$SKILLS_DIR/$name/SKILL.md" || true)" updated)")
		[ "$got" = "$want" ] || fail "$README_FILE: $name row says \"$got\" but metadata.updated is $want; run: sh scripts/skills.sh stamp $name"
	done
	return 0
}

run_check() {
	use_git=0
	[ "${1-}" = "--git" ] && use_git=1
	read_version
	# 地球上最早进入新一天的时区，避免跨时区协作时把当天日期误判为未来。
	latest_today=$(TZ=Etc/GMT-14 date +%Y-%m-%d)
	names=$(list_skills)
	[ -n "$names" ] || die "$SKILLS_DIR: no skills found"
	count=0
	for name in $names; do
		check_skill "$name"
		count=$((count + 1))
	done
	check_marketplace
	check_readme
	[ "$problems" -eq 0 ] || exit 1
	printf 'ok: %s skills, AlonaBot %s\n' "$count" "$version"
}

# write_if_changed 用 $2 的内容覆盖 $1（保留权限），有改动时输出文件名。
write_if_changed() {
	if cmp -s "$1" "$2"; then
		rm -f "$2"
	else
		cat "$2" >"$1"
		rm -f "$2"
		printf '%s\n' "$1"
	fi
}

# stamp_skill 更新一个技能的 metadata 与全部可见行，输出改动过的文件。
stamp_skill() {
	dir=$SKILLS_DIR/$1
	[ -f "$dir/SKILL.md" ] || die "$dir: missing SKILL.md"
	head=$(frontmatter "$dir/SKILL.md") || die "$dir/SKILL.md: bad frontmatter"
	[ -n "$(meta_value "$head" alonabot-version)" ] && [ -n "$(meta_value "$head" updated)" ] ||
		die "$dir/SKILL.md: metadata needs alonabot-version and updated lines"
	for md in $(find "$dir" -type f -name '*.md' | sort); do
		tmp=$md.stamp.tmp
		awk -v v="$version" -v d="$today" -v skill="$([ "${md##*/}" = SKILL.md ] && echo 1 || echo 0)" '
			skill && NR == 1 && $0 == "---" { in_head = 1; print; next }
			in_head && $0 == "---" { in_head = 0; print; next }
			in_head && /^metadata:/ { in_meta = 1; print; next }
			in_head && /^[^ ]/ { in_meta = 0 }
			in_meta && /^  alonabot-version:/ { print "  alonabot-version: \"" v "\""; next }
			in_meta && /^  updated:/ { print "  updated: \"" d "\""; next }
			/^> AlonaBot [^ ]+ · 最后更新 [^ ]+$/ { print "> AlonaBot " v " · 最后更新 " d; next }
			{ print }' "$md" >"$tmp"
		write_if_changed "$md" "$tmp"
	done
}

# stamp_readme 把 README 技能表中这些技能的日期改为今天。
stamp_readme() {
	tmp=$README_FILE.stamp.tmp
	awk -v d="$today" -v names=" $* " '
		/^\| \[[a-z0-9-]+\]\(skills\/[a-z0-9-]+\/SKILL\.md\) \|.*\| [0-9-]+ \|$/ {
			n = $0; sub(/^\| \[/, "", n); sub(/\].*/, "", n)
			if (index(names, " " n " ")) { sub(/\| [0-9-]+ \|$/, "| " d " |") }
		}
		{ print }' "$README_FILE" >"$tmp"
	write_if_changed "$README_FILE" "$tmp"
}

# staged_skills 输出暂存区中有改动的技能名。
staged_skills() {
	git diff --cached --name-only --diff-filter=ACMR -- "$SKILLS_DIR" | awk -F/ 'NF >= 3 { print $2 }' | sort -u
}

run_stamp() {
	read_version
	today=$(date +%Y-%m-%d)
	staged=0
	case "${1-}" in
	--staged)
		staged=1
		names=$(staged_skills)
		[ -n "$names" ] || return 0
		# 文件另有未暂存改动时拒绝，避免 git add 把它们一起提交。
		for name in $names; do
			dirty=$(git diff --name-only -- "$SKILLS_DIR/$name")
			[ -z "$dirty" ] || die "$SKILLS_DIR/$name has unstaged changes; stage them first, or run: sh scripts/skills.sh stamp $name"
		done
		[ -z "$(git diff --name-only -- "$README_FILE")" ] ||
			die "$README_FILE has unstaged changes; stage them first so the skills table can be re-staged"
		;;
	--all) names=$(list_skills) ;;
	"") die "name the skills to stamp, or use --all or --staged" ;;
	*) names=$* ;;
	esac
	changed=""
	for name in $names; do
		changed="$changed $(stamp_skill "$name" | tr '\n' ' ')"
	done
	# shellcheck disable=SC2086 # names 是空白分隔的技能名列表
	changed="$changed $(stamp_readme $names)"
	for f in $changed; do
		printf 'stamped %s: AlonaBot %s · %s\n' "$f" "$version" "$today"
	done
	if [ "$staged" = 1 ] && [ -n "$(printf '%s' "$changed" | tr -d ' ')" ]; then
		# shellcheck disable=SC2086
		git add -- $changed
	fi
	return 0
}

case "${1-}" in
check)
	shift
	run_check "$@"
	;;
stamp)
	shift
	run_stamp "$@"
	;;
*)
	echo "usage: sh scripts/skills.sh check [--git] | stamp [--all | --staged | skill...]" >&2
	exit 2
	;;
esac

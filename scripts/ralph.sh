#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/ralph.sh [options]

Automates "Slice N" execution in sequence:
1) implement with Codex
2) run tests
3) review with Codex + fix rounds
4) commit and update GitHub issue

Options:
  --repo OWNER/REPO         GitHub repo (default: current gh repo)
  --base-branch BRANCH      Base branch to start each slice from (default: main)
  --start N                 First slice number to process (default: 0)
  --end N                   Last slice number to process (default: no upper bound)
  --test-cmd CMD            Test command (default: "go test ./...")
  --max-fix-rounds N        Max review->fix loops per slice (default: 2)
  --model MODEL             Codex model (optional)
  --codex-cmd CMD_OR_PATH   Codex executable or absolute path (default: auto-detect)
  --push                    Push slice branch to origin
  --create-pr               Create PR after successful slice (implies --push)
  --close-issue             Close issue after successful slice
  --dry-run                 Print actions without executing mutating steps
  -h, --help                Show help

Env overrides:
  RALPH_REPO
  RALPH_BASE_BRANCH
  RALPH_START
  RALPH_END
  RALPH_TEST_CMD
  RALPH_MAX_FIX_ROUNDS
  RALPH_MODEL
  RALPH_CODEX_CMD
USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

resolve_codex_cmd() {
  local candidate=""

  if [[ -n "${CODEX_CMD:-}" ]]; then
    if command -v "$CODEX_CMD" >/dev/null 2>&1; then
      command -v "$CODEX_CMD"
      return 0
    fi
    if [[ -x "$CODEX_CMD" ]]; then
      printf '%s\n' "$CODEX_CMD"
      return 0
    fi
    return 1
  fi

  if command -v codex >/dev/null 2>&1; then
    command -v codex
    return 0
  fi

  for candidate in "$HOME"/.vscode/extensions/openai.chatgpt-*/bin/*/codex; do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  return 1
}

log() {
  printf '[ralph] %s\n' "$*"
}

run() {
  if (( DRY_RUN )); then
    printf '[dry-run] '
    printf '%q ' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

run_shell() {
  if (( DRY_RUN )); then
    printf '[dry-run] bash -lc %q\n' "$1"
    return 0
  fi
  bash -lc "$1"
}

slugify() {
  local s="$1"
  s="$(printf '%s' "$s" | tr '[:upper:]' '[:lower:]')"
  s="$(printf '%s' "$s" | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g')"
  printf '%s' "$s"
}

REPO="${RALPH_REPO:-}"
BASE_BRANCH="${RALPH_BASE_BRANCH:-main}"
START="${RALPH_START:-0}"
END="${RALPH_END:-}"
TEST_CMD="${RALPH_TEST_CMD:-go test ./...}"
MAX_FIX_ROUNDS="${RALPH_MAX_FIX_ROUNDS:-2}"
MODEL="${RALPH_MODEL:-}"
CODEX_CMD="${RALPH_CODEX_CMD:-}"
PUSH=0
CREATE_PR=0
CLOSE_ISSUE=0
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO="$2"
      shift 2
      ;;
    --base-branch)
      BASE_BRANCH="$2"
      shift 2
      ;;
    --start)
      START="$2"
      shift 2
      ;;
    --end)
      END="$2"
      shift 2
      ;;
    --test-cmd)
      TEST_CMD="$2"
      shift 2
      ;;
    --max-fix-rounds)
      MAX_FIX_ROUNDS="$2"
      shift 2
      ;;
    --model)
      MODEL="$2"
      shift 2
      ;;
    --codex-cmd)
      CODEX_CMD="$2"
      shift 2
      ;;
    --push)
      PUSH=1
      shift
      ;;
    --create-pr)
      CREATE_PR=1
      PUSH=1
      shift
      ;;
    --close-issue)
      CLOSE_ISSUE=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

require_cmd gh
require_cmd jq
require_cmd git

if ! CODEX_BIN="$(resolve_codex_cmd)"; then
  echo "Missing required command: codex" >&2
  echo "Install Codex CLI, or pass --codex-cmd /absolute/path/to/codex, or set RALPH_CODEX_CMD." >&2
  exit 1
fi

if [[ -z "$REPO" ]]; then
  REPO="$(gh repo view --json nameWithOwner --jq '.nameWithOwner')"
fi

if ! [[ "$START" =~ ^[0-9]+$ ]]; then
  echo "--start must be a non-negative integer" >&2
  exit 1
fi
if [[ -n "$END" ]] && ! [[ "$END" =~ ^[0-9]+$ ]]; then
  echo "--end must be a non-negative integer" >&2
  exit 1
fi
if ! [[ "$MAX_FIX_ROUNDS" =~ ^[0-9]+$ ]]; then
  echo "--max-fix-rounds must be a non-negative integer" >&2
  exit 1
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Run this script inside a git repo." >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  if (( DRY_RUN )); then
    log "Working tree is dirty; continuing because --dry-run is enabled."
  else
    echo "Working tree must be clean before running ralph." >&2
    exit 1
  fi
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

review_schema="$tmpdir/review-schema.json"
cat > "$review_schema" <<'JSON'
{
  "type": "object",
  "required": ["approved", "summary", "blocking_findings"],
  "additionalProperties": false,
  "properties": {
    "approved": {"type": "boolean"},
    "summary": {"type": "string"},
    "blocking_findings": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["severity", "issue", "fix"],
        "additionalProperties": false,
        "properties": {
          "severity": {
            "type": "string",
            "enum": ["critical", "high", "medium", "low"]
          },
          "file": {"type": "string"},
          "line": {"type": "integer"},
          "issue": {"type": "string"},
          "fix": {"type": "string"}
        }
      }
    }
  }
}
JSON

codex_exec_base=("$CODEX_BIN" -a never -s workspace-write exec)
if [[ -n "$MODEL" ]]; then
  codex_exec_base+=(--model "$MODEL")
fi

log "Repo: $REPO"
log "Base branch: $BASE_BRANCH"
log "Slice range: $START to ${END:-max}"
log "Test command: $TEST_CMD"
log "Codex command: $CODEX_BIN"

run git switch "$BASE_BRANCH"
run git pull --ff-only

current_ref="$BASE_BRANCH"
previous_branch=""

slices_tsv="$tmpdir/slices.tsv"
gh issue list \
  --repo "$REPO" \
  --state open \
  --limit 200 \
  --json number,title,url \
  --jq '.
    | map(select(.title | test("^Slice [0-9]+:")))
    | map(. + {slice: ((.title | capture("^Slice (?<n>[0-9]+):").n) | tonumber)})
    | sort_by(.slice)
    | .[]
    | [.slice, .number, .title, .url]
    | @tsv' > "$slices_tsv"

if [[ ! -s "$slices_tsv" ]]; then
  log "No open Slice issues found in $REPO"
  exit 0
fi

while IFS=$'\t' read -r slice_num issue_num issue_title issue_url; do
  if (( slice_num < START )); then
    continue
  fi
  if [[ -n "$END" ]] && (( slice_num > END )); then
    continue
  fi

  log "---"
  log "Processing Slice $slice_num (#$issue_num): $issue_title"

  issue_json="$tmpdir/issue-$slice_num.json"
  gh issue view "$issue_num" --repo "$REPO" --json number,title,body,url > "$issue_json"

  title="$(jq -r '.title' "$issue_json")"
  body="$(jq -r '.body' "$issue_json")"
  clean_title="${title#*: }"
  branch="slice-$(printf '%02d' "$slice_num")-$(slugify "$clean_title")"
  branch="${branch:0:72}"

  if git show-ref --verify --quiet "refs/heads/$branch"; then
    run git switch "$branch"
  else
    run git switch -c "$branch" "$current_ref"
  fi

  run gh issue comment "$issue_num" --repo "$REPO" \
    --body "Ralph started implementation for Slice $slice_num on branch \`$branch\`."

  implement_prompt="$tmpdir/implement-$slice_num.txt"
  cat > "$implement_prompt" <<EOF
You are implementing one thin vertical slice.

Slice: $title
Issue URL: $issue_url

Requirements:
$body

Instructions:
- Implement only this slice.
- Keep changes minimal and cohesive.
- Add or update tests so behavior is validated.
- Run the project's test command and ensure it passes.
- Do not skip failing tests; fix root causes.
- End with a brief summary of files changed and tests run.
EOF

  log "Implementing slice via Codex"
  if (( DRY_RUN )); then
    log "[dry-run] codex exec implement prompt for slice $slice_num"
  else
    "${codex_exec_base[@]}" - < "$implement_prompt"
  fi

  log "Running tests"
  run_shell "$TEST_CMD"

  review_ok=0
  for round in $(seq 0 "$MAX_FIX_ROUNDS"); do
    review_json="$tmpdir/review-$slice_num-round-$round.json"
    review_prompt="$tmpdir/review-$slice_num-round-$round.txt"

    cat > "$review_prompt" <<EOF
Review the current git working tree changes for this slice.

Slice: $title
Requirements:
$body

Focus on blocking issues only:
- bugs
- behavior regressions
- incorrect or missing tests
- violations of the slice requirements

Return JSON only via the provided schema.
Set approved=true only when there are zero blocking findings.
EOF

    log "Review round $round"
    if (( DRY_RUN )); then
      log "[dry-run] codex exec review for slice $slice_num round $round"
      review_ok=1
      break
    else
      "${codex_exec_base[@]}" \
        --output-schema "$review_schema" \
        --output-last-message "$review_json" \
        - < "$review_prompt"
    fi

    approved="$(jq -r '.approved' "$review_json")"
    findings_count="$(jq -r '.blocking_findings | length' "$review_json")"

    if [[ "$approved" == "true" ]] && [[ "$findings_count" == "0" ]]; then
      log "Review passed"
      review_ok=1
      break
    fi

    if (( round == MAX_FIX_ROUNDS )); then
      log "Review failed after $MAX_FIX_ROUNDS fix rounds"
      jq -r '.summary' "$review_json" >&2 || true
      jq -r '.blocking_findings[]? | "- [\(.severity)] \(.file // "<unknown>"):\(.line // 0) \(.issue) | fix: \(.fix)"' "$review_json" >&2 || true
      exit 1
    fi

    findings_file="$tmpdir/findings-$slice_num-round-$round.txt"
    jq -r '.blocking_findings[] | "- [\(.severity)] \(.file // "<unknown>"):\(.line // 0) \(.issue)\n  Fix: \(.fix)"' "$review_json" > "$findings_file"

    fix_prompt="$tmpdir/fix-$slice_num-round-$round.txt"
    cat > "$fix_prompt" <<EOF
Fix all blocking review findings for this slice.

Slice: $title
Findings:
$(cat "$findings_file")

Instructions:
- Implement fixes for every listed finding.
- Update/add tests as needed.
- Keep scope strictly within this slice.
EOF

    log "Applying fixes for review findings"
    "${codex_exec_base[@]}" - < "$fix_prompt"

    log "Re-running tests after fixes"
    run_shell "$TEST_CMD"
  done

  if (( review_ok == 0 )); then
    log "Unexpected review state for slice $slice_num"
    exit 1
  fi

  if [[ -n "$(git status --porcelain)" ]]; then
    run git add -A
    commit_msg="feat(slice-$slice_num): $clean_title"
    run git commit -m "$commit_msg"
  else
    log "No file changes detected for slice $slice_num; skipping commit"
  fi

  if (( PUSH )); then
    run git push -u origin "$branch"
  fi

  pr_url=""
  if (( CREATE_PR )); then
    pr_base="$BASE_BRANCH"
    if [[ -n "$previous_branch" ]]; then
      pr_base="$previous_branch"
    fi

    pr_body_file="$tmpdir/pr-$slice_num.md"
    cat > "$pr_body_file" <<EOF
Implements **$title**.

- Issue: $issue_url
- Slice: $slice_num
- Test command: \`$TEST_CMD\`

This PR was generated by \`scripts/ralph.sh\`.
EOF

    if (( DRY_RUN )); then
      log "[dry-run] gh pr create for $branch"
      pr_url="<dry-run-pr-url>"
    else
      pr_url="$(gh pr create --repo "$REPO" --base "$pr_base" --head "$branch" --title "$title" --body-file "$pr_body_file")"
      log "Created PR: $pr_url"
    fi
  fi

  done_comment="Ralph completed Slice $slice_num on branch \`$branch\`."
  if [[ -n "$pr_url" ]]; then
    done_comment+=" PR: $pr_url"
  fi

  run gh issue comment "$issue_num" --repo "$REPO" --body "$done_comment"

  if (( CLOSE_ISSUE )); then
    run gh issue close "$issue_num" --repo "$REPO" --comment "Completed by ralph automation on branch \`$branch\`."
  fi

  previous_branch="$branch"
  current_ref="$branch"
done < "$slices_tsv"

log "All requested slices completed."

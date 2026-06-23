# cast-sim.sh — SIMULATION shim for the `cast ai annotate` demo GIF only.
# Sourced by demo/tapes/ai-annotate.tape so the *typed* command on screen is the
# real `cast ai annotate --dry-run`, while the output below is faithful but
# fully invented (the real command needs a Groq API key + network, so it can't
# run in a reproducible recording). Every other `cast …` call falls through to
# the real binary on PATH.
cast() {
	if [ "$1" = "ai" ] && [ "$2" = "annotate" ]; then
		local g=$'\033[38;2;80;250;123m'   # add  (#50FA7B)
		local d=$'\033[38;2;122;136;184m'  # dim  (#7A88B8)
		local x=$'\033[0m'
		printf '%sprovider=groq model=llama-3.3-70b-versatile targets=3 elapsed=742ms%s\n' "$d" "$x"
		printf '%s@@ migrate @@%s\n'                                            "$d" "$x"
		printf '%s+## migrate: Run pending database migrations [tags=db]%s\n'   "$g" "$x"
		printf '%s migrate:%s\n'                                                "$d" "$x"
		printf '%s \t@goose up%s\n'                                             "$d" "$x"
		printf '%s@@ seed @@%s\n'                                               "$d" "$x"
		printf '%s+## seed: Load demo fixtures into the database [tags=db,dev]%s\n' "$g" "$x"
		printf '%s seed:%s\n'                                                   "$d" "$x"
		printf '%s \t@go run ./cmd/seed%s\n'                                    "$d" "$x"
		printf '%s@@ release @@%s\n'                                            "$d" "$x"
		printf '%s+## release: Tag and publish a new release [tags=release]%s\n' "$g" "$x"
		printf '%s release:%s\n'                                                "$d" "$x"
		printf '%s \t@./scripts/release.sh%s\n'                                 "$d" "$x"
		printf '\n%sskipped 1 target(s):%s\n'                                   "$d" "$x"
		printf '%s  setup — recipe is empty, cannot infer purpose%s\n'          "$d" "$x"
		return 0
	fi
	command cast "$@"
}

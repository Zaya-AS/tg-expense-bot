#!/bin/sh
set -eu

project_dir=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
cd "$project_dir"

backup_dir=${1:-"$project_dir/backups"}
umask 077
mkdir -p "$backup_dir"

stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_file="$backup_dir/expenses-$stamp-$$.dump"
temp_file=$(mktemp "$backup_file.tmp.XXXXXX")
trap 'rm -f "$temp_file"' 0 HUP INT TERM

docker compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$temp_file"
mv "$temp_file" "$backup_file"
trap - 0 HUP INT TERM
printf 'Backup saved: %s\n' "$backup_file"

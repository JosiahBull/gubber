#!/usr/bin/env bash

subcommand=$1
repo=$2
days=$3
output=$4

if [ -z "$subcommand" ]; then
    echo "Usage: gubber.sh <subcommand> <repo> <days>"
    exit 1
fi

if [ -z "$repo" ]; then
    echo "Usage: gubber.sh <subcommand> <repo> <days>"
    exit 1
fi

if [ -z "$days" ]; then
    echo "Usage: gubber.sh <subcommand> <repo> <days>"
    exit 1
fi

case $subcommand in
    "restore")
        echo "Attempting to restore $repo from $days days ago"
        cp "backT-${days}/${repo}" "${output}"
        ;;
    *)
        echo "Usage: gubber.sh <subcommand> <repo> <days>"
        exit 1
        ;;
esac
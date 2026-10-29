#!/bin/sh
case "$1" in
Username*) exec echo "x-oauth-basic" ;;
*) exec echo "$X_OAUTH_BASIC_TOKEN" ;;
esac

#!/bin/sh


# --batch to prevent interactive command
# --yes to assume "yes" for questions
gpg --quiet --batch --yes --decrypt --passphrase="$LARGE_SECRET_PASSPHRASE" \
--output ~/.sshkey ./.github/secrets/sshkey.gpg
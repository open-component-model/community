#!/bin/bash

# 1. Migrates labels based upon the mapping file 'label_mappings.json'. 
#    They are either simply renamed or if the destination already exists, 
#    the new label is added to the issue and the old one is detached.
# 
# 2. Removes ALL unused labels from a GitHub repository!
#
# Usage: ./migrate-labels.sh [ghp_TOKEN]
#

set -e

# Required environment variables
GITHUB_TOKEN="$1"
GITHUB_ORG="open-component-model"
GITHUB_REPO="ocm-workpackages"

# Array to store all labels
labels=()

# Function to URL encode a string
urlencode() {
    local string="$1"
    local encoded=""
    local length="${#string}"
    for (( i = 0; i < length; i++ )); do
        local c="${string:i:1}"
        case "$c" in
            [a-zA-Z0-9.~_-]) encoded+="$c" ;;
            *) encoded+=$(printf '%%%02X' "'$c") ;;
        esac
    done
    echo "$encoded"
}

# Function to fetch all labels with pagination
fetch_all_labels() {
    local page=1
    local per_page=100

    while :; do
        response=$(curl -sSL -H "Accept: application/vnd.github.v3+json" \
                          -H "Authorization: token ${GITHUB_TOKEN}" \
                          "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/labels?per_page=${per_page}&page=${page}")

        # Extract label names and add to the labels array
        label_names=$(echo "$response" | jq -r '.[].name' | sed 's/ /%20/g')
        if [ -z "$label_names" ]; then
            break
        fi
        labels+=($label_names)
        
        # Check if we have reached the last page
        if [ $(echo "$response" | jq 'length') -lt $per_page ]; then
            break
        fi

        page=$((page + 1))
    done
}

# Function to check if a label is used in any issue or pull request
is_label_used() {
    local label="$1"

    response=$(curl -sSL -H "Accept: application/vnd.github.v3+json" \
                -H "Authorization: token ${GITHUB_TOKEN}" \
                "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/issues?labels=${label}")

    # Check if there are any issues or pull requests with the label
    if [ $(echo "$response" | jq 'length') -gt 0 ]; then
        return 0
    fi
    return 1
}

# Function to delete a label from the repository
delete_label() {
    local label="$(urlencode $1)"

    response=$(curl -sSL -X DELETE -H "Accept: application/vnd.github.v3+json" \
                -H "Authorization: token ${GITHUB_TOKEN}" \
                "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/labels/${label}")

    if [ $? -eq 0 ]; then
        echo "Deleted unused label '$label'."
    else
        echo "Failed to delete label '$label'."
    fi
}

# Function to add a label to an issue
add_label_to_issue() {
    local issue_number="$1"
    local label="$2"

    response=$(curl -sSL -X POST -H "Accept: application/vnd.github.v3+json" \
                -H "Authorization: token ${GITHUB_TOKEN}" \
                -d "{\"labels\": [\"${label}\"]}" \
                "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/issues/${issue_number}/labels")

    if [ $? -eq 0 ]; then
        echo "Added label '${label}' to issue #${issue_number}."
    else
        echo "Failed to add label '${label}' to issue #${issue_number}."
    fi
}

# Function to remove a label from an issue
remove_label_from_issue() {
    local issue_number="$1"
    local label="$2"
    local encoded_label=$(urlencode "$label")

    response=$(curl -sSL -X DELETE -H "Accept: application/vnd.github.v3+json" \
                -H "Authorization: token ${GITHUB_TOKEN}" \
                "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/issues/${issue_number}/labels/${encoded_label}")

    if [ $? -eq 0 ]; then
        echo "Removed label '${label}' from issue #${issue_number}."
    else
        echo "Failed to remove label '${label}' from issue #${issue_number}."
    fi
}

# Function to migrate issues from old label to new label
migrate_label() {
    local old_label="$1"
    local new_label="$2"
    local encoded_old_label=$(urlencode "$old_label")

    # FIXME: paging?
    response=$(curl -sSL -H "Accept: application/vnd.github.v3+json" \
                -H "Authorization: token ${GITHUB_TOKEN}" \
                "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/issues?labels=${encoded_old_label}")

    issue_numbers=$(echo "$response" | jq -r '.[].number')

    for issue_number in $issue_numbers; do
        add_label_to_issue "$issue_number" "$new_label"
        remove_label_from_issue "$issue_number" "$old_label"
    done
}

# Function to rename a label in the repository
rename_label() {
    local old_label="$1"
    local encoded_old_label=$(urlencode "$old_label")
    local new_label="$2"

    response=$(curl -sSL -X PATCH -H "Accept: application/vnd.github.v3+json" \
                -H "Authorization: token ${GITHUB_TOKEN}" \
                -d "{\"new_name\": \"${new_label}\"}" \
                "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/labels/${encoded_old_label}")

    if [ $? -eq 0 ]; then
        if echo "$response" | jq -e '.errors' > /dev/null; then
            echo "Failed to rename label '${old_label}' to '${new_label}': $(echo "$response" | jq -r '.errors[0].code')"
            if echo "$response" | jq -e '.errors[0].code' | grep -q 'already_exists'; then
                echo "Label '${new_label}' already exists. Adding '${new_label}' to issues with '${old_label}' and removing '${old_label}'."
                migrate_label "$old_label" "$new_label"
            fi
        else
            echo "Renamed label '${old_label}' to '${new_label}'."
        fi
    else
        echo "Failed to rename label '${old_label}' to '${new_label}'."
    fi

    echo "$response"
}

# Read the label mappings from the JSON file
label_mappings=$(jq -r 'to_entries[] | "\(.key);\(.value)"' label_mappings.json)

# Iterate over the label mappings and rename/migrate the labels
while IFS=";" read -r old_label new_label; do
    rename_label "$old_label" "$new_label"
done <<< "$label_mappings"

# Fetch all labels from the repository and start deleting unused labels
fetch_all_labels
# Iterate over all labels and check if they are used
for label in ${labels[@]}; do
    if ! is_label_used "${label}"; then
        delete_label "${label}"
    fi
done

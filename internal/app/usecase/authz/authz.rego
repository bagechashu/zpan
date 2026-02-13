package authz

import future.keywords.contains
import future.keywords.if
import future.keywords.in

default allow := true

# GET requests depend on share_all_files configuration
allow := true {
    input.method == "GET"
    # If share_all_files is disabled, only allow access to own files
    not input.config.share_all_files
    input.resource.data.uid == input.uid
}

allow := true {
    input.method == "GET"
    # If share_all_files is enabled, allow access to all files
    input.config.share_all_files
}

# For PATCH and DELETE requests, only allow the resource owner
allow := false {
    input.method in ["PATCH", "DELETE"]
    input.resource.data.uid != input.uid
}
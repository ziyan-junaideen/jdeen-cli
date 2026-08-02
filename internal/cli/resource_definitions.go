package cli

func postDefinition() resourceDefinition {
	return resourceDefinition{
		use: "posts", apiType: "posts", short: "Manage blog posts",
		relationships: map[string]bool{"author": false, "categories": true, "banner_upload": false, "comments": true},
		addCreate:     addPostCreate, addUpdate: addPostUpdate,
	}
}

func categoryDefinition() resourceDefinition {
	return resourceDefinition{
		use: "categories", apiType: "categories", short: "Manage post categories",
		relationships: map[string]bool{"posts": true},
		addCreate:     addCategoryCreate, addUpdate: addCategoryUpdate,
	}
}

func uploadDefinition() resourceDefinition {
	return resourceDefinition{
		use: "uploads", apiType: "uploads", short: "Manage uploaded images",
		relationships: map[string]bool{"banner_posts": true},
		addCreate:     addUploadCreate, addUpdate: addUploadUpdate,
	}
}

func commentDefinition() resourceDefinition {
	return resourceDefinition{
		use: "comments", aliases: []string{"post-comments", "post_comments"}, apiType: "post_comments", short: "Manage post comments",
		relationships: map[string]bool{"post": false, "user": false, "parent_comment": false, "replies": true},
		addCreate:     addCommentCreate, addUpdate: addCommentUpdate,
	}
}

func userDefinition() resourceDefinition {
	return resourceDefinition{
		use: "users", apiType: "users", short: "Inspect public users", readOnly: true,
		relationships: map[string]bool{"profile_upload": false},
	}
}

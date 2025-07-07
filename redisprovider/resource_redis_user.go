package redisprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRedisUser() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRedisUserCreate,
		ReadContext:   resourceRedisUserRead,
		UpdateContext: resourceRedisUserUpdate,
		DeleteContext: resourceRedisUserDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceRedisUserImport,
		},
		Schema: map[string]*schema.Schema{
			"username": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"password": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"keys": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Key patterns the user can access (e.g. 'cache:*')",
			},
			"commands": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Commands or command categories the user can execute",
			},
			"channels": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Pub/Sub channel patterns the user can access",
			},
			"reset_keys": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to reset keys before applying new ones",
			},
			"reset_channels": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to reset channels before applying new ones",
			},
			"reset_commands": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to reset commands before applying new ones",
			},
			"acl_string": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The full ACL string for the user",
			},
		},
	}
}

func resourceRedisUserCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	cfg := m.(*ProviderConfig)
	username := d.Get("username").(string)

	// Check if user exists
	users, err := cfg.RedisClient.ACLList(ctx).Result()
	if err != nil {
		return diag.FromErr(err)
	}

	userExists := false
	for _, user := range users {
		if user == username {
			userExists = true
			break
		}
	}

	if !userExists {
		// Create user with minimal permissions
		_, err := cfg.RedisClient.Do(ctx, "ACL", "SETUSER", username).Result()
		if err != nil {
			return diag.Errorf("Failed to create Redis user '%s': %s", username, err)
		}
	}

	d.SetId(username)

	// Apply ACL settings
	return updateUserACL(ctx, d, cfg)
}

func resourceRedisUserRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	cfg := m.(*ProviderConfig)
	username := d.Id()

	// Get user ACL
	aclInfoCmd := cfg.RedisClient.Do(ctx, "ACL", "GETUSER", username)
	if aclInfoCmd.Err() != nil {
		// If user doesn't exist, remove from state
		if strings.Contains(aclInfoCmd.Err().Error(), "User not found") {
			d.SetId("")
			return nil
		}
		return diag.FromErr(aclInfoCmd.Err())
	}

	aclInfoSlice, err := aclInfoCmd.Slice()
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("username", username)

	// Parse ACL rules
	enabled := true
	keys := []string{}
	commands := []string{}
	channels := []string{}

	// Find the rules in the ACL info
	for i := 0; i < len(aclInfoSlice); i += 2 {
		fieldName, ok := aclInfoSlice[i].(string)
		if !ok {
			continue
		}
		if fieldName == "flags" {
			flags, ok := aclInfoSlice[i+1].([]interface{})
			if ok {
				for _, flag := range flags {
					flagStr, ok := flag.(string)
					if ok && flagStr == "off" {
						enabled = false
					}
				}
			}
		} else if fieldName == "keys" {
			keyPatterns, ok := aclInfoSlice[i+1].([]interface{})
			if ok {
				for _, pattern := range keyPatterns {
					patternStr, ok := pattern.(string)
					if ok {
						// Remove the ~ prefix if present
						if strings.HasPrefix(patternStr, "~") {
							keys = append(keys, patternStr[1:])
						} else {
							keys = append(keys, patternStr)
						}
					}
				}
			}
		} else if fieldName == "channels" {
			channelPatterns, ok := aclInfoSlice[i+1].([]interface{})
			if ok {
				for _, pattern := range channelPatterns {
					patternStr, ok := pattern.(string)
					if ok {
						// Remove the & prefix if present
						if strings.HasPrefix(patternStr, "&") {
							channels = append(channels, patternStr[1:])
						} else {
							channels = append(channels, patternStr)
						}
					}
				}
			}
		} else if fieldName == "commands" {
			commandStr, ok := aclInfoSlice[i+1].(string)
			if ok {
				commands = append(commands, commandStr)
			}
		}
	}

	d.Set("enabled", enabled)
	d.Set("keys", keys)
	d.Set("commands", commands)
	d.Set("channels", channels)

	// Get the full ACL string
	aclString, err := cfg.RedisClient.Do(ctx, "ACL", "LIST").StringSlice()
	if err != nil {
		return diag.FromErr(err)
	}

	for _, acl := range aclString {
		parts := strings.SplitN(acl, " ", 2)
		if len(parts) == 2 && parts[0] == username {
			d.Set("acl_string", parts[1])
			break
		}
	}

	// Password is write-only, we don't read it back

	return nil
}

func resourceRedisUserUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	cfg := m.(*ProviderConfig)

	return updateUserACL(ctx, d, cfg)
}

func resourceRedisUserDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	cfg := m.(*ProviderConfig)
	username := d.Id()

	// Don't delete the default user
	if username == "default" {
		return diag.Errorf("Cannot delete the 'default' user")
	}

	_, err := cfg.RedisClient.Do(ctx, "ACL", "DELUSER", username).Result()
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func resourceRedisUserImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	username := d.Id()
	cfg := m.(*ProviderConfig)

	// Check if user exists
	aclInfoCmd := cfg.RedisClient.Do(ctx, "ACL", "GETUSER", username)
	if aclInfoCmd.Err() != nil {
		return nil, fmt.Errorf("Redis user '%s' not found", username)
	}

	d.Set("username", username)

	// Get the full ACL string
	aclString, err := cfg.RedisClient.Do(ctx, "ACL", "LIST").StringSlice()
	if err != nil {
		return nil, err
	}

	for _, acl := range aclString {
		parts := strings.SplitN(acl, " ", 2)
		if len(parts) == 2 && parts[0] == username {
			d.Set("acl_string", parts[1])
			break
		}
	}

	return []*schema.ResourceData{d}, nil
}

func updateUserACL(ctx context.Context, d *schema.ResourceData, cfg *ProviderConfig) diag.Diagnostics {
	username := d.Get("username").(string)

	// Build ACL command arguments
	args := []interface{}{"ACL", "SETUSER", username}

	// Set enabled/disabled status
	if enabled, ok := d.GetOk("enabled"); ok {
		if enabled.(bool) {
			args = append(args, "on")
		} else {
			args = append(args, "off")
		}
	}

	// Set password if provided
	if password, ok := d.GetOk("password"); ok {
		args = append(args, fmt.Sprintf(">%s", password.(string)))
	} else {
		// If no password is provided, set nopass
		args = append(args, "nopass")
	}

	// Reset keys if requested
	if reset, ok := d.GetOk("reset_keys"); ok && reset.(bool) {
		args = append(args, "resetkeys")
	}

	// Reset channels if requested
	if reset, ok := d.GetOk("reset_channels"); ok && reset.(bool) {
		args = append(args, "resetchannels")
	}

	// Reset commands if requested
	if reset, ok := d.GetOk("reset_commands"); ok && reset.(bool) {
		args = append(args, "resetcommands")
	}

	// Add key patterns
	if keys, ok := d.GetOk("keys"); ok {
		for _, key := range keys.([]interface{}) {
			keyStr := key.(string)
			// Ensure key pattern has ~ prefix for Redis ACL syntax
			if !strings.HasPrefix(keyStr, "~") {
				args = append(args, fmt.Sprintf("~%s", keyStr))
			} else {
				args = append(args, keyStr)
			}
		}
	}

	// Add command permissions
	if commands, ok := d.GetOk("commands"); ok {
		for _, cmd := range commands.([]interface{}) {
			cmdStr := cmd.(string)

			// Handle command categories and individual commands correctly
			if strings.HasPrefix(cmdStr, "@") {
				// It's a command category without + or - prefix, add + prefix
				if !strings.HasPrefix(cmdStr, "+@") && !strings.HasPrefix(cmdStr, "-@") {
					cmdStr = "+@" + cmdStr[1:]
				}
			} else if !strings.HasPrefix(cmdStr, "+") && !strings.HasPrefix(cmdStr, "-") {
				// It's an individual command without + or - prefix, add + prefix
				cmdStr = "+" + cmdStr
			}

			args = append(args, cmdStr)
		}
	}

	// Add channel permissions
	if channels, ok := d.GetOk("channels"); ok {
		for _, channel := range channels.([]interface{}) {
			channelStr := channel.(string)
			// Ensure channel pattern has & prefix for Redis ACL syntax
			if !strings.HasPrefix(channelStr, "&") {
				args = append(args, fmt.Sprintf("&%s", channelStr))
			} else {
				args = append(args, channelStr)
			}
		}
	}

	// Apply the ACL changes
	_, err := cfg.RedisClient.Do(ctx, args...).Result()
	if err != nil {
		return diag.Errorf("Failed to update Redis user '%s': %s", username, err)
	}

	return resourceRedisUserRead(ctx, d, cfg)
}

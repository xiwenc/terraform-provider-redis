package redisprovider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccRedisUser_basic(t *testing.T) {
	resourceName := "redis_user.test"
	username := "testuser"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckRedisUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRedisUserConfig_basic(username),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "username", username),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "keys.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "keys.0", "test:*"),
					resource.TestCheckResourceAttr(resourceName, "commands.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "commands.0", "+@read"),
				),
			},
			{
				// Test importing the resource
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"password", // Password is write-only and not returned by Redis
					"reset_keys",
					"reset_channels",
					"reset_commands",
				},
			},
		},
	})
}

func TestAccRedisUser_update(t *testing.T) {
	resourceName := "redis_user.test"
	username := "testuser"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckRedisUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRedisUserConfig_basic(username),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "username", username),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "keys.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "keys.0", "test:*"),
					resource.TestCheckResourceAttr(resourceName, "commands.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "commands.0", "+@read"),
				),
			},
			{
				Config: testAccRedisUserConfig_update(username),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "username", username),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "keys.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "keys.0", "test:*"),
					resource.TestCheckResourceAttr(resourceName, "keys.1", "cache:*"),
					resource.TestCheckResourceAttr(resourceName, "commands.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "commands.0", "+@read"),
					resource.TestCheckResourceAttr(resourceName, "commands.1", "+@write"),
					resource.TestCheckResourceAttr(resourceName, "channels.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "channels.0", "news:*"),
				),
			},
		},
	})
}

func TestAccRedisUser_full(t *testing.T) {
	resourceName := "redis_user.test"
	username := "testuser"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckRedisUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRedisUserConfig_full(username),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRedisUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "username", username),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "keys.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "keys.0", "test:*"),
					resource.TestCheckResourceAttr(resourceName, "keys.1", "cache:*"),
					resource.TestCheckResourceAttr(resourceName, "commands.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "commands.0", "+@read"),
					resource.TestCheckResourceAttr(resourceName, "commands.1", "+@write"),
					resource.TestCheckResourceAttr(resourceName, "commands.2", "-@dangerous"),
					resource.TestCheckResourceAttr(resourceName, "channels.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "channels.0", "news:*"),
					resource.TestCheckResourceAttr(resourceName, "channels.1", "events:*"),
					resource.TestCheckResourceAttr(resourceName, "reset_keys", "true"),
					resource.TestCheckResourceAttr(resourceName, "reset_channels", "true"),
					resource.TestCheckResourceAttr(resourceName, "reset_commands", "true"),
				),
			},
		},
	})
}

func testAccCheckRedisUserExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no resource ID is set")
		}

		config := testAccProvider.Meta().(*ProviderConfig)
		username := rs.Primary.ID

		// Check if the user exists
		cmd := config.RedisClient.Do(context.Background(), "ACL", "GETUSER", username)
		if cmd.Err() != nil {
			return fmt.Errorf("error checking user existence: %s", cmd.Err())
		}

		return nil
	}
}

func testAccCheckRedisUserDestroy(s *terraform.State) error {
	config := testAccProvider.Meta().(*ProviderConfig)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "redis_user" {
			continue
		}

		username := rs.Primary.ID
		cmd := config.RedisClient.Do(context.Background(), "ACL", "GETUSER", username)

		// If the user still exists, this is an error
		if cmd.Err() == nil {
			return fmt.Errorf("Redis user %s still exists", username)
		}

		// If the error is not "user not found", this is an unexpected error
		if cmd.Err() != nil && !isUserNotFoundError(cmd.Err()) {
			return fmt.Errorf("unexpected error checking user existence: %s", cmd.Err())
		}
	}

	return nil
}

// Helper function to check if error is "user not found"
func isUserNotFoundError(err error) bool {
	return err != nil && err.Error() == "ERR User not found"
}

func testAccRedisUserConfig_basic(username string) string {
	return fmt.Sprintf(`
resource "redis_user" "test" {
  username = "%s"
  password = "testpassword"
  enabled  = true
  keys     = ["test:*"]
  commands = ["+@read"]
}
`, username)
}

func testAccRedisUserConfig_update(username string) string {
	return fmt.Sprintf(`
resource "redis_user" "test" {
  username = "%s"
  password = "updatedpassword"
  enabled  = true
  keys     = ["test:*", "cache:*"]
  commands = ["+@read", "+@write"]
  channels = ["news:*"]
}
`, username)
}

func testAccRedisUserConfig_full(username string) string {
	return fmt.Sprintf(`
resource "redis_user" "test" {
  username       = "%s"
  password       = "testpassword"
  enabled        = true
  keys           = ["test:*", "cache:*"]
  commands       = ["+@read", "+@write", "-@dangerous"]
  channels       = ["news:*", "events:*"]
  reset_keys     = true
  reset_channels = true
  reset_commands = true
}
`, username)
}

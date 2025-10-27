package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ha "github.com/mkelcik/go-ha-client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Global Home Assistant client
var client *ha.Client

// Input types
type ListEntitiesInput struct {
}

type GetServicesInput struct {
}

type CallServiceInput struct {
	Domain   string `json:"domain" jsonschema:"the domain of the service (e.g., 'switch', 'light')"`
	Service  string `json:"service" jsonschema:"the service name (e.g., 'turn_on', 'turn_off')"`
	EntityID string `json:"entity_id" jsonschema:"the entity ID (e.g., 'switch.switch_1')"`
}

// Output types
type Entity struct {
	EntityID     string      `json:"entity_id" jsonschema:"the entity ID"`
	State        string      `json:"state" jsonschema:"current state"`
	FriendlyName interface{} `json:"friendly_name" jsonschema:"friendly name if available"`
	Domain       string      `json:"domain" jsonschema:"domain of the entity"`
}

type ListEntitiesOutput struct {
	Entities []Entity `json:"entities" jsonschema:"list of all entities"`
	Count    int      `json:"count" jsonschema:"number of entities"`
}

type ServiceField struct {
	Description string `json:"description" jsonschema:"field description"`
	Example     string `json:"example,omitempty" jsonschema:"example value"`
	Required    bool   `json:"required,omitempty" jsonschema:"whether the field is required"`
}

type Service struct {
	Domain string                  `json:"domain" jsonschema:"service domain"`
	Name   string                  `json:"name" jsonschema:"service name"`
	Fields map[string]ServiceField `json:"fields" jsonschema:"service fields"`
}

type GetServicesOutput struct {
	Services []Service `json:"services" jsonschema:"list of all available services"`
	Count    int       `json:"count" jsonschema:"number of services"`
}

type CallServiceOutput struct {
	Success bool   `json:"success" jsonschema:"whether the call was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

// ListEntities returns all entities in Home Assistant
func ListEntities(ctx context.Context, req *mcp.CallToolRequest, input ListEntitiesInput) (
	*mcp.CallToolResult,
	ListEntitiesOutput,
	error,
) {
	states, err := client.GetStates(ctx)
	if err != nil {
		return nil, ListEntitiesOutput{}, fmt.Errorf("failed to get states: %w", err)
	}

	var entities []Entity
	for _, state := range states {
		// Extract domain from entity ID
		data := strings.Split(state.EntityId, ".")
		domain := ""
		if len(data) > 0 {
			domain = data[0]
		}

		// Extract friendly name if available
		friendlyName := state.Attributes["friendly_name"]

		entity := Entity{
			EntityID:     state.EntityId,
			State:        state.State,
			FriendlyName: friendlyName,
			Domain:       domain,
		}
		entities = append(entities, entity)
	}

	output := ListEntitiesOutput{
		Entities: entities,
		Count:    len(entities),
	}

	return nil, output, nil
}

// GetServices returns all available services in Home Assistant
func GetServices(ctx context.Context, req *mcp.CallToolRequest, input GetServicesInput) (
	*mcp.CallToolResult,
	GetServicesOutput,
	error,
) {
	services, err := client.GetServices(ctx)
	if err != nil {
		return nil, GetServicesOutput{}, fmt.Errorf("failed to get services: %w", err)
	}

	var result []Service
	for _, s := range services {
		for serviceName, serviceInfo := range s.Services {
			// Convert service fields to our format
			fields := make(map[string]ServiceField)
			for fieldName, fieldInfo := range serviceInfo.Fields {
				field := ServiceField{
					Description: fieldInfo.Description,
				}

				// Set example if available (convert to string if needed)
				if fieldInfo.Example != nil {
					switch v := fieldInfo.Example.(type) {
					case string:
						field.Example = v
					default:
						field.Example = fmt.Sprintf("%v", v)
					}
				}

				// Set required flag if available
				if fieldInfo.Selector != nil {
					field.Required = true // Set based on selector if needed
				}

				fields[fieldName] = field
			}

			service := Service{
				Domain: s.Domain,
				Name:   serviceName,
				Fields: fields,
			}
			result = append(result, service)
		}
	}

	output := GetServicesOutput{
		Services: result,
		Count:    len(result),
	}

	return nil, output, nil
}

// CallService calls a service in Home Assistant
func CallService(ctx context.Context, req *mcp.CallToolRequest, input CallServiceInput) (
	*mcp.CallToolResult,
	CallServiceOutput,
	error,
) {
	// Prepare the service command
	cmd := ha.DefaultServiceCmd{
		Domain:   input.Domain,
		Service:  input.Service,
		EntityId: input.EntityID,
	}

	// Call the service
	_, err := client.CallService(ctx, cmd)
	if err != nil {
		return nil, CallServiceOutput{
			Success: false,
			Message: fmt.Sprintf("Failed to call service: %v", err),
		}, nil
	}

	output := CallServiceOutput{
		Success: true,
		Message: fmt.Sprintf("Successfully called %s.%s on entity %s", input.Domain, input.Service, input.EntityID),
	}

	return nil, output, nil
}

func main() {
	// Get configuration from environment variables
	token := os.Getenv("HA_TOKEN")
	if token == "" {
		log.Fatal("HA_TOKEN environment variable is required")
	}

	host := os.Getenv("HA_HOST")
	if host == "" {
		host = "http://localhost:8123"
	}

	// Create Home Assistant client
	client = ha.NewClient(
		ha.ClientConfig{
			Token: token,
			Host:  host,
		},
		&http.Client{
			Timeout: 30 * time.Second,
		},
	)

	// Test connection
	if err := client.Ping(context.Background()); err != nil {
		log.Printf("Warning: Could not ping Home Assistant instance: %v", err)
	} else {
		log.Println("Connected to Home Assistant instance")
	}

	// Create MCP server with Home Assistant tools
	server := mcp.NewServer(&mcp.Implementation{Name: "homeassistant", Version: "v1.0.0"}, nil)

	// Register tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_entities",
		Description: "List all entities in Home Assistant and their current states",
	}, ListEntities)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_services",
		Description: "Get all available services in Home Assistant",
	}, GetServices)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "call_service",
		Description: "Call a service in Home Assistant (e.g., turn_on, turn_off, toggle)",
	}, CallService)

	// Run the server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

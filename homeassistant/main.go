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

type SearchEntitiesInput struct {
	Keyword string `json:"keyword" jsonschema:"search keyword to match in entity ID, domain, state, or friendly name"`
}

type SearchServicesInput struct {
	Keyword string `json:"keyword" jsonschema:"search keyword to match in service domain or name"`
}

// Output types
type Entity struct {
	EntityID     string      `json:"entity_id" jsonschema:"the entity ID"`
	State        string      `json:"state" jsonschema:"current state"`
	FriendlyName interface{} `json:"friendly_name" jsonschema:"friendly name if available"`
	Domain       string      `json:"domain" jsonschema:"domain of the entity"`
}

// EntitySummary is a compact view for listing; use search_entities for full details.
type EntitySummary struct {
	EntityID     string      `json:"entity_id" jsonschema:"the entity ID"`
	Domain       string      `json:"domain" jsonschema:"domain of the entity"`
	FriendlyName interface{} `json:"friendly_name" jsonschema:"friendly name if available"`
}

type ListEntitiesOutput struct {
	Entities []EntitySummary `json:"entities" jsonschema:"compact list of entity IDs and domains; use search_entities to get full details"`
	Count    int             `json:"count" jsonschema:"number of entities"`
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

// ServiceSummary is a compact view for listing; use search_services for full details.
type ServiceSummary struct {
	Domain string `json:"domain" jsonschema:"service domain"`
	Name   string `json:"name" jsonschema:"service name"`
}

type GetServicesOutput struct {
	Services []ServiceSummary `json:"services" jsonschema:"compact list of service domain and name; use search_services to get full details including fields"`
	Count    int              `json:"count" jsonschema:"number of services"`
}

type CallServiceOutput struct {
	Success bool   `json:"success" jsonschema:"whether the call was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type SearchEntitiesOutput struct {
	Entities []Entity `json:"entities" jsonschema:"list of matching entities"`
	Count    int      `json:"count" jsonschema:"number of matching entities"`
}

type SearchServicesOutput struct {
	Services []Service `json:"services" jsonschema:"list of matching services"`
	Count    int       `json:"count" jsonschema:"number of matching services"`
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

	entities := []EntitySummary{}
	for _, state := range states {
		data := strings.Split(state.EntityId, ".")
		domain := ""
		if len(data) > 0 {
			domain = data[0]
		}
		entities = append(entities, EntitySummary{
			EntityID:     state.EntityId,
			Domain:       domain,
			FriendlyName: state.Attributes["friendly_name"],
		})
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

	result := []ServiceSummary{}
	for _, s := range services {
		for serviceName := range s.Services {
			result = append(result, ServiceSummary{
				Domain: s.Domain,
				Name:   serviceName,
			})
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

// SearchEntities searches for entities matching the keyword
func SearchEntities(ctx context.Context, req *mcp.CallToolRequest, input SearchEntitiesInput) (
	*mcp.CallToolResult,
	SearchEntitiesOutput,
	error,
) {
	states, err := client.GetStates(ctx)
	if err != nil {
		return nil, SearchEntitiesOutput{}, fmt.Errorf("failed to get states: %w", err)
	}

	keyword := strings.ToLower(input.Keyword)
	matchingEntities := []Entity{}

	for _, state := range states {
		// Extract domain from entity ID
		data := strings.Split(state.EntityId, ".")
		domain := ""
		if len(data) > 0 {
			domain = data[0]
		}

		// Check if keyword matches in entity ID
		entityIDMatch := strings.Contains(strings.ToLower(state.EntityId), keyword)

		// Check if keyword matches in domain
		domainMatch := strings.Contains(strings.ToLower(domain), keyword)

		// Check if keyword matches in state
		stateMatch := strings.Contains(strings.ToLower(state.State), keyword)

		// Check if keyword matches in friendly name
		friendlyNameMatch := false
		if friendlyName, ok := state.Attributes["friendly_name"].(string); ok {
			friendlyNameMatch = strings.Contains(strings.ToLower(friendlyName), keyword)
		}

		// If keyword matches in any field, include this entity
		if entityIDMatch || domainMatch || stateMatch || friendlyNameMatch {
			friendlyName := state.Attributes["friendly_name"]
			entity := Entity{
				EntityID:     state.EntityId,
				State:        state.State,
				FriendlyName: friendlyName,
				Domain:       domain,
			}
			matchingEntities = append(matchingEntities, entity)
		}
	}

	output := SearchEntitiesOutput{
		Entities: matchingEntities,
		Count:    len(matchingEntities),
	}

	return nil, output, nil
}

// SearchServices searches for services matching the keyword
func SearchServices(ctx context.Context, req *mcp.CallToolRequest, input SearchServicesInput) (
	*mcp.CallToolResult,
	SearchServicesOutput,
	error,
) {
	services, err := client.GetServices(ctx)
	if err != nil {
		return nil, SearchServicesOutput{}, fmt.Errorf("failed to get services: %w", err)
	}

	keyword := strings.ToLower(input.Keyword)
	result := []Service{}

	for _, s := range services {
		// Check if keyword matches in domain
		domainMatch := strings.Contains(strings.ToLower(s.Domain), keyword)

		for serviceName, serviceInfo := range s.Services {
			// Check if keyword matches in service name
			serviceNameMatch := strings.Contains(strings.ToLower(serviceName), keyword)

			// If keyword matches in domain or service name, include this service
			if domainMatch || serviceNameMatch {
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
						field.Required = true
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
	}

	output := SearchServicesOutput{
		Services: result,
		Count:    len(result),
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
		Description: "List all entities in Home Assistant (compact: entity_id and domain only). Use search_entities with a keyword to get full details (state, friendly_name).",
	}, ListEntities)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_services",
		Description: "Get all available services in Home Assistant (compact: domain and name only). Use search_services with a keyword to get full details including fields.",
	}, GetServices)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "call_service",
		Description: "Call a service in Home Assistant (e.g., turn_on, turn_off, toggle)",
	}, CallService)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_entities",
		Description: "Search for entities in Home Assistant by keyword (searches entity ID, domain, state, friendly name). Returns full details: entity_id, state, friendly_name, domain.",
	}, SearchEntities)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_services",
		Description: "Search for services in Home Assistant by keyword (searches domain and name). Returns full details including service fields.",
	}, SearchServices)

	// Run the server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handlers", func() {
	var tempDir string
	var filePath string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "todo-handler-test-*")
		Expect(err).NotTo(HaveOccurred())
		filePath = filepath.Join(tempDir, "todos.json")
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Context("Handler factories", func() {
		It("should add TODO and return correct output in admin mode", func() {
			storage := NewFileStorage(filePath)
			service := NewService(storage)
			setGlobalService(service)
			handler := NewAddTODOHandler(true)

			_, output, err := handler(context.Background(), nil, AddTODOInput{
				ID:       "todo-1",
				Title:    "Test TODO",
				Assignee: "agent1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.ID).To(Equal("todo-1"))
			Expect(output.Title).To(Equal("Test TODO"))
			Expect(output.Assignee).To(Equal("agent1"))
			Expect(output.Status).To(Equal("pending"))
		})

		It("should reject AddTODO when not in admin mode", func() {
			storage := NewFileStorage(filePath)
			service := NewService(storage)
			setGlobalService(service)
			handler := NewAddTODOHandler(false)

			_, _, err := handler(context.Background(), nil, AddTODOInput{
				ID:    "todo-1",
				Title: "Test",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("admin mode"))
		})

		It("should update TODO status in admin mode", func() {
			storage := NewFileStorage(filePath)
			service := NewService(storage)
			setGlobalService(service)
			addHandler := NewAddTODOHandler(true)
			updateHandler := NewUpdateTODOStatusHandler(true)

			_, addOut, _ := addHandler(context.Background(), nil, AddTODOInput{ID: "todo-1", Title: "Test"})
			_, updateOut, err := updateHandler(context.Background(), nil, UpdateTODOStatusInput{
				ID:     addOut.ID,
				Status: "in_progress",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(updateOut.Success).To(BeTrue())
		})

		It("should list all TODOs", func() {
			storage := NewFileStorage(filePath)
			service := NewService(storage)
			setGlobalService(service)
			addHandler := NewAddTODOHandler(true)

			_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-1", Title: "Test 1"})
			_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-2", Title: "Test 2"})

			_, output, err := ListTODOs(context.Background(), nil, struct{}{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(Equal(2))
		})
	})

	Context("Dependency handlers", func() {
		var storage *FileStorage
		var service *Service
		var addHandler func(context.Context, *mcp.CallToolRequest, AddTODOInput) (*mcp.CallToolResult, AddTODOOutput, error)

		BeforeEach(func() {
			storage = NewFileStorage(filePath)
			service = NewService(storage)
			setGlobalService(service)
			addHandler = NewAddTODOHandler(true)

			// Create base TODOs
			_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-1", Title: "A"})
			_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-2", Title: "B"})
		})

		It("should add dependency", func() {
			handler := NewAddTODODependencyHandler(true)
			_, output, err := handler(context.Background(), nil, AddTODODependencyInput{
				ID:        "todo-2",
				DependsOn: "todo-1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Success).To(BeTrue())
		})

		It("should get ready TODOs", func() {
			_, outB, _ := addHandler(context.Background(), nil, AddTODOInput{
				ID:        "todo-3",
				Title:     "C",
				DependsOn: []string{"todo-1"},
			})
			_ = service.UpdateStatus("todo-1", "done")

			_, output, err := GetReadyTODOs(context.Background(), nil, GetReadyTODOsInput{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Items).To(ContainElement(HaveField("ID", outB.ID)))
		})

		It("should get blocked TODOs", func() {
			_, _, _ = addHandler(context.Background(), nil, AddTODOInput{
				ID:        "todo-3",
				Title:     "C",
				DependsOn: []string{"todo-1"},
			})

			_, output, err := GetBlockedTODOs(context.Background(), nil, GetBlockedTODOsInput{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.Count).To(BeNumerically(">", 0))
		})

		It("should get TODO dependencies", func() {
			_ = service.AddDependency("todo-2", "todo-1")
			_, output, err := GetTODODependencies(context.Background(), nil, GetTODODependenciesInput{
				ID:         "todo-2",
				Transitive: false,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(output.DirectCount).To(Equal(1))
		})
	})

	Context("Permission checks with factories", func() {
		var storage *FileStorage
		var service *Service

		BeforeEach(func() {
			storage = NewFileStorage(filePath)
			service = NewService(storage)
			setGlobalService(service)
		})

		Context("Admin-only handlers", func() {
			It("should reject AddTODO when not in admin mode", func() {
				handler := NewAddTODOHandler(false)
				_, _, err := handler(context.Background(), nil, AddTODOInput{
					ID:    "todo-1",
					Title: "Test",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("admin mode"))
			})

			It("should allow AddTODO when in admin mode", func() {
				handler := NewAddTODOHandler(true)
				_, output, err := handler(context.Background(), nil, AddTODOInput{
					ID:    "todo-1",
					Title: "Test",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.ID).To(Equal("todo-1"))
			})

			It("should reject RemoveTODO when not in admin mode", func() {
				addHandler := NewAddTODOHandler(true)
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-1", Title: "Test"})

				removeHandler := NewRemoveTODOHandler(false)
				_, _, err := removeHandler(context.Background(), nil, RemoveTODOInput{ID: "todo-1"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("admin mode"))
			})

			It("should reject AddTODODependency when not in admin mode", func() {
				addHandler := NewAddTODOHandler(true)
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-1", Title: "A"})
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-2", Title: "B"})

				depHandler := NewAddTODODependencyHandler(false)
				_, _, err := depHandler(context.Background(), nil, AddTODODependencyInput{
					ID:        "todo-2",
					DependsOn: "todo-1",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("admin mode"))
			})

			It("should reject RemoveTODODependency when not in admin mode", func() {
				addHandler := NewAddTODOHandler(true)
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-1", Title: "A"})
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-2", Title: "B"})
				_ = service.AddDependency("todo-2", "todo-1")

				removeDepHandler := NewRemoveTODODependencyHandler(false)
				_, _, err := removeDepHandler(context.Background(), nil, RemoveTODODependencyInput{
					ID:        "todo-2",
					DependsOn: "todo-1",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("admin mode"))
			})

			It("should reject UpdateTODOAssignee when not in admin mode", func() {
				addHandler := NewAddTODOHandler(true)
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{ID: "todo-1", Title: "Test"})

				assignHandler := NewUpdateTODOAssigneeHandler(false)
				_, _, err := assignHandler(context.Background(), nil, UpdateTODOAssigneeInput{
					ID:       "todo-1",
					Assignee: "agent1",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("admin mode"))
			})
		})

		Context("UpdateTODOStatus permissions", func() {
			BeforeEach(func() {
				addHandler := NewAddTODOHandler(true)
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{
					ID:       "todo-1",
					Title:    "Test",
					Assignee: "agent1",
				})
				_, _, _ = addHandler(context.Background(), nil, AddTODOInput{
					ID:       "todo-2",
					Title:    "Test 2",
					Assignee: "agent2",
				})
			})

			It("should allow updating any TODO in admin mode without agent name", func() {
				handler := NewUpdateTODOStatusHandler(true)
				_, output, err := handler(context.Background(), nil, UpdateTODOStatusInput{
					ID:     "todo-1",
					Status: "in_progress",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.Success).To(BeTrue())
			})

			It("should allow updating TODO assigned to agent in agent mode", func() {
				handler := NewUpdateTODOStatusHandler(false)
				_, output, err := handler(context.Background(), nil, UpdateTODOStatusInput{
					ID:        "todo-1",
					Status:    "in_progress",
					AgentName: "agent1",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.Success).To(BeTrue())
			})

			It("should reject updating TODO not assigned to agent in agent mode", func() {
				handler := NewUpdateTODOStatusHandler(false)
				_, output, err := handler(context.Background(), nil, UpdateTODOStatusInput{
					ID:        "todo-1",
					Status:    "in_progress",
					AgentName: "agent2",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.Success).To(BeFalse())
				Expect(output.Message).To(ContainSubstring("not assigned to agent"))
			})

			It("should reject updating TODO without agent name in agent mode", func() {
				handler := NewUpdateTODOStatusHandler(false)
				_, output, err := handler(context.Background(), nil, UpdateTODOStatusInput{
					ID:     "todo-1",
					Status: "in_progress",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(output.Success).To(BeFalse())
				Expect(output.Message).To(ContainSubstring("agent_name is required"))
			})
		})
	})
})


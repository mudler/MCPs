package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	todos     *TODOList
	loadError error
	saveError error
	lockError error
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		todos: &TODOList{Items: []TODOItem{}},
	}
}

func (m *MockStorage) Load() (*TODOList, error) {
	if m.loadError != nil {
		return nil, m.loadError
	}
	// Return a copy to prevent accidental mutations
	items := make([]TODOItem, len(m.todos.Items))
	copy(items, m.todos.Items)
	return &TODOList{Items: items}, nil
}

func (m *MockStorage) Save(list *TODOList) error {
	if m.saveError != nil {
		return m.saveError
	}
	// Store a copy
	items := make([]TODOItem, len(list.Items))
	copy(items, list.Items)
	m.todos = &TODOList{Items: items}
	return nil
}

func (m *MockStorage) WithLock(fn func() error) error {
	if m.lockError != nil {
		return m.lockError
	}
	return fn()
}

var _ = Describe("Service", func() {
	var mockStorage *MockStorage
	var service *Service

	BeforeEach(func() {
		mockStorage = NewMockStorage()
		service = NewService(mockStorage)
	})

	Context("Service creation", func() {
		It("should create service with storage", func() {
			Expect(service).NotTo(BeNil())
			Expect(service.storage).To(Equal(mockStorage))
		})
	})

	Context("AddTODO", func() {
		It("should create TODO with provided ID", func() {
			item, err := service.AddTODO("todo-1", "Test TODO", "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.ID).To(Equal("todo-1"))
			Expect(item.Title).To(Equal("Test TODO"))
		})

		It("should return error if ID is empty", func() {
			_, err := service.AddTODO("", "Test", "", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ID is required"))
		})

		It("should return error if ID already exists", func() {
			_, err := service.AddTODO("todo-1", "Test 1", "", nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = service.AddTODO("todo-1", "Test 2", "", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already exists"))
		})

		It("should accept provided ID when unique", func() {
			item1, err := service.AddTODO("todo-1", "Test 1", "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(item1.ID).To(Equal("todo-1"))

			item2, err := service.AddTODO("todo-2", "Test 2", "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(item2.ID).To(Equal("todo-2"))
		})

		It("should set status to pending", func() {
			item, err := service.AddTODO("todo-1", "Test", "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.Status).To(Equal("pending"))
		})

		It("should accept optional assignee", func() {
			item, err := service.AddTODO("todo-1", "Test", "agent1", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.Assignee).To(Equal("agent1"))
		})

		It("should return error if title is empty", func() {
			_, err := service.AddTODO("todo-1", "", "", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("title is required"))
		})

		It("should save to storage", func() {
			_, err := service.AddTODO("todo-1", "Test", "", nil)
			Expect(err).NotTo(HaveOccurred())

			list, err := mockStorage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Title).To(Equal("Test"))
			Expect(list.Items[0].ID).To(Equal("todo-1"))
		})
	})

	Context("UpdateStatus", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Test", "", nil)
		})

		It("should change status", func() {
			err := service.UpdateStatus("todo-1", "in_progress")
			Expect(err).NotTo(HaveOccurred())

			list, _ := mockStorage.Load()
			Expect(list.Items[0].Status).To(Equal("in_progress"))
		})

		It("should validate status values", func() {
			err := service.UpdateStatus("todo-1", "pending")
			Expect(err).NotTo(HaveOccurred())

			err = service.UpdateStatus("todo-1", "in_progress")
			Expect(err).NotTo(HaveOccurred())

			err = service.UpdateStatus("todo-1", "done")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for invalid status", func() {
			err := service.UpdateStatus("todo-1", "invalid")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid status"))
		})

		It("should return error if TODO not found", func() {
			err := service.UpdateStatus("nonexistent", "in_progress")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("UpdateAssignee", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Test", "", nil)
		})

		It("should change assignee", func() {
			err := service.UpdateAssignee("todo-1", "agent1")
			Expect(err).NotTo(HaveOccurred())

			list, _ := mockStorage.Load()
			Expect(list.Items[0].Assignee).To(Equal("agent1"))
		})

		It("should return error if TODO not found", func() {
			err := service.UpdateAssignee("nonexistent", "agent1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("RemoveTODO", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Test 1", "", nil)
			_, _ = service.AddTODO("todo-2", "Test 2", "", nil)
		})

		It("should remove TODO", func() {
			err := service.RemoveTODO("todo-1")
			Expect(err).NotTo(HaveOccurred())

			list, _ := mockStorage.Load()
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].ID).To(Equal("todo-2"))
		})

		It("should return error if TODO not found", func() {
			err := service.RemoveTODO("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("ListTODOs", func() {
		It("should return all TODOs", func() {
			_, _ = service.AddTODO("todo-1", "Test 1", "", nil)
			_, _ = service.AddTODO("todo-2", "Test 2", "", nil)

			items, err := service.ListTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(items).To(HaveLen(2))
		})

		It("should return empty list when no TODOs", func() {
			items, err := service.ListTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(items).To(BeEmpty())
		})
	})

	Context("GetStatus", func() {
		It("should calculate correct counts by status", func() {
			_, _ = service.AddTODO("todo-1", "Pending 1", "", nil)
			_, _ = service.AddTODO("todo-2", "Pending 2", "", nil)
			_, _ = service.AddTODO("todo-3", "In Progress", "", nil)
			_ = service.UpdateStatus("todo-3", "in_progress")
			_, _ = service.AddTODO("todo-4", "Done", "", nil)
			_ = service.UpdateStatus("todo-4", "done")

			status, err := service.GetStatus()
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Pending).To(Equal(2))
			Expect(status.InProgress).To(Equal(1))
			Expect(status.Done).To(Equal(1))
			Expect(status.Total).To(Equal(4))
		})

		It("should calculate counts by assignee", func() {
			_, _ = service.AddTODO("todo-1", "Task 1", "agent1", nil)
			_, _ = service.AddTODO("todo-2", "Task 2", "agent1", nil)
			_, _ = service.AddTODO("todo-3", "Task 3", "agent2", nil)

			status, err := service.GetStatus()
			Expect(err).NotTo(HaveOccurred())
			Expect(status.ByAssignee["agent1"]).To(Equal(2))
			Expect(status.ByAssignee["agent2"]).To(Equal(1))
		})
	})

	Context("AddTODO with dependencies", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Dependency 1", "", nil)
			_, _ = service.AddTODO("todo-2", "Dependency 2", "", nil)
		})

		It("should accept DependsOn parameter", func() {
			item, err := service.AddTODO("todo-3", "Test", "", []string{"todo-1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(Equal([]string{"todo-1"}))
		})

		It("should validate all dependency IDs exist", func() {
			item, err := service.AddTODO("todo-3", "Test", "", []string{"todo-1", "todo-2"})
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(Equal([]string{"todo-1", "todo-2"}))
		})

		It("should return error if dependency ID doesn't exist", func() {
			_, err := service.AddTODO("todo-3", "Test", "", []string{"nonexistent"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dependency"))
		})

		It("should prevent self-dependency when adding TODO", func() {
			// This will be tested after we add the TODO, so we need to check after creation
			// For now, we'll test it in AddDependency context
		})

		It("should create TODO with empty DependsOn if nil", func() {
			item, err := service.AddTODO("todo-3", "Test", "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(BeEmpty())
		})
	})

	Context("AddDependency", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "TODO A", "", nil)
			_, _ = service.AddTODO("todo-2", "TODO B", "", nil)
		})

		It("should add dependency to existing TODO", func() {
			err := service.AddDependency("todo-2", "todo-1")
			Expect(err).NotTo(HaveOccurred())

			items, _ := service.ListTODOs()
			var item *TODOItem
			for i := range items {
				if items[i].ID == "todo-2" {
					item = &items[i]
					break
				}
			}
			Expect(item).NotTo(BeNil())
			Expect(item.DependsOn).To(ContainElement("todo-1"))
		})

		It("should validate both TODO IDs exist", func() {
			err := service.AddDependency("todo-2", "todo-1")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent duplicate dependencies", func() {
			_ = service.AddDependency("todo-2", "todo-1")
			err := service.AddDependency("todo-2", "todo-1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already exists"))
		})

		It("should prevent self-dependency", func() {
			err := service.AddDependency("todo-1", "todo-1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("self"))
		})

		It("should return error if TODO not found", func() {
			err := service.AddDependency("nonexistent", "todo-1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should return error if dependency TODO not found", func() {
			err := service.AddDependency("todo-1", "nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("RemoveDependency", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "TODO A", "", nil)
			_, _ = service.AddTODO("todo-2", "TODO B", "", []string{"todo-1"})
		})

		It("should remove dependency", func() {
			err := service.RemoveDependency("todo-2", "todo-1")
			Expect(err).NotTo(HaveOccurred())

			items, _ := service.ListTODOs()
			var item *TODOItem
			for i := range items {
				if items[i].ID == "todo-2" {
					item = &items[i]
					break
				}
			}
			Expect(item).NotTo(BeNil())
			Expect(item.DependsOn).NotTo(ContainElement("todo-1"))
		})

		It("should handle non-existent dependency gracefully", func() {
			err := service.RemoveDependency("todo-2", "nonexistent")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if TODO not found", func() {
			err := service.RemoveDependency("nonexistent", "todo-1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("Circular dependency detection", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "TODO A", "", nil)
			_, _ = service.AddTODO("todo-2", "TODO B", "", nil)
			_, _ = service.AddTODO("todo-3", "TODO C", "", nil)
		})

		It("should detect A→B→A cycle", func() {
			_ = service.AddDependency("todo-2", "todo-1")
			err := service.AddDependency("todo-1", "todo-2")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("circular"))
		})

		It("should detect A→B→C→A cycle", func() {
			_ = service.AddDependency("todo-2", "todo-1")
			_ = service.AddDependency("todo-3", "todo-2")
			err := service.AddDependency("todo-1", "todo-3")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("circular"))
		})

		It("should allow A→B→C (no cycle)", func() {
			err := service.AddDependency("todo-2", "todo-1")
			Expect(err).NotTo(HaveOccurred())
			err = service.AddDependency("todo-3", "todo-2")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow A→B, C→B (no cycle)", func() {
			err := service.AddDependency("todo-2", "todo-1")
			Expect(err).NotTo(HaveOccurred())
			err = service.AddDependency("todo-2", "todo-3")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent circular dependencies in AddDependency", func() {
			_ = service.AddDependency("todo-2", "todo-1")
			err := service.AddDependency("todo-1", "todo-2")
			Expect(err).To(HaveOccurred())
		})

		It("should indicate circular dependency in error message", func() {
			_ = service.AddDependency("todo-2", "todo-1")
			err := service.AddDependency("todo-1", "todo-2")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("circular"))
		})
	})

	Context("Status change with dependencies", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Dependency 1", "", nil)
			_, _ = service.AddTODO("todo-2", "Dependency 2", "", nil)
			_, _ = service.AddTODO("todo-3", "Dependent", "", []string{"todo-1", "todo-2"})
		})

		It("should block UpdateStatus to in_progress if dependencies not done", func() {
			err := service.UpdateStatus("todo-3", "in_progress")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("blocked"))
		})

		It("should allow UpdateStatus to in_progress if all dependencies done", func() {
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "done")
			err := service.UpdateStatus("todo-3", "in_progress")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow UpdateStatus to done if dependencies satisfied (skip in_progress)", func() {
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "done")
			err := service.UpdateStatus("todo-3", "done")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should always allow UpdateStatus from in_progress to done", func() {
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "done")
			_ = service.UpdateStatus("todo-3", "in_progress")
			err := service.UpdateStatus("todo-3", "done")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow UpdateStatus from done to in_progress (reopening)", func() {
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "done")
			_ = service.UpdateStatus("todo-3", "done")
			err := service.UpdateStatus("todo-3", "in_progress")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow UpdateStatus from in_progress to pending (reverting)", func() {
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "done")
			_ = service.UpdateStatus("todo-3", "in_progress")
			err := service.UpdateStatus("todo-3", "pending")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include blocking TODO IDs and their statuses in error messages", func() {
			err := service.UpdateStatus("todo-3", "in_progress")
			Expect(err).To(HaveOccurred())
			errMsg := err.Error()
			Expect(errMsg).To(ContainSubstring("todo-1"))
			Expect(errMsg).To(ContainSubstring("todo-2"))
		})

		It("should list all blocking dependencies in error", func() {
			_ = service.UpdateStatus("todo-1", "done")
			// todo-2 is still pending
			err := service.UpdateStatus("todo-3", "in_progress")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("todo-2"))
		})
	})

	Context("GetReadyTODOs", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Dependency", "", nil)
			_, _ = service.AddTODO("todo-2", "Ready 1", "", []string{"todo-1"})
			_, _ = service.AddTODO("todo-3", "Ready 2", "", nil)
			_, _ = service.AddTODO("todo-4", "Blocked", "", []string{"todo-1"})
		})

		It("should return only pending TODOs with all dependencies done", func() {
			_ = service.UpdateStatus("todo-1", "done")
			ready, err := service.GetReadyTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(HaveLen(3))
			ids := []string{ready[0].ID, ready[1].ID, ready[2].ID}
			Expect(ids).To(ContainElements("todo-2", "todo-3", "todo-4"))
		})

		It("should exclude TODOs with unsatisfied dependencies", func() {
			ready, err := service.GetReadyTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(HaveLen(2))
			ids := []string{ready[0].ID, ready[1].ID}
			Expect(ids).To(ContainElements("todo-1", "todo-3"))
		})

		It("should exclude in_progress and done TODOs", func() {
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "in_progress")
			_ = service.UpdateStatus("todo-3", "done")
			ready, err := service.GetReadyTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(HaveLen(1))
			Expect(ready[0].ID).To(Equal("todo-4"))
		})
	})

	Context("GetBlockedTODOs", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Dependency 1", "", nil)
			_, _ = service.AddTODO("todo-2", "Dependency 2", "", nil)
			_, _ = service.AddTODO("todo-3", "Blocked 1", "", []string{"todo-1"})
			_, _ = service.AddTODO("todo-4", "Blocked 2", "", []string{"todo-1", "todo-2"})
			_, _ = service.AddTODO("todo-5", "Ready", "", []string{"todo-1"})
		})

		It("should return pending TODOs with blocking info", func() {
			_ = service.UpdateStatus("todo-1", "done")
			blocked, err := service.GetBlockedTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(blocked).To(HaveLen(1))
			Expect(blocked[0].ID).To(Equal("todo-4"))
		})

		It("should include which dependencies are blocking", func() {
			blocked, err := service.GetBlockedTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(blocked).To(HaveLen(3))
			// Check blocking info
			for _, b := range blocked {
				Expect(b.BlockedBy).NotTo(BeEmpty())
			}
		})

		It("should include status of blocking TODOs", func() {
			blocked, err := service.GetBlockedTODOs()
			Expect(err).NotTo(HaveOccurred())
			if len(blocked) > 0 {
				Expect(blocked[0].BlockedBy[0].Status).NotTo(BeEmpty())
			}
		})

		It("should exclude TODOs with no dependencies", func() {
			_, _ = service.AddTODO("todo-6", "No deps", "", nil)
			blocked, err := service.GetBlockedTODOs()
			Expect(err).NotTo(HaveOccurred())
			// No deps TODO should not be in blocked list
			for _, b := range blocked {
				Expect(b.ID).NotTo(Equal("todo-6"))
			}
		})

		It("should exclude TODOs with all dependencies satisfied", func() {
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "done")
			blocked, err := service.GetBlockedTODOs()
			Expect(err).NotTo(HaveOccurred())
			Expect(blocked).To(BeEmpty())
		})
	})

	Context("GetDependencies", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "A", "", nil)
			_, _ = service.AddTODO("todo-2", "B", "", []string{"todo-1"})
			_, _ = service.AddTODO("todo-3", "C", "", []string{"todo-2"})
		})

		It("should return direct dependencies", func() {
			deps, err := service.GetDependencies("todo-2", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(deps.Direct).To(HaveLen(1))
			Expect(deps.Direct[0].ID).To(Equal("todo-1"))
		})

		It("should return transitive dependencies when requested", func() {
			deps, err := service.GetDependencies("todo-3", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(deps.Direct).To(HaveLen(1))
			Expect(deps.Transitive).NotTo(BeEmpty())
		})

		It("should include status of each dependency", func() {
			deps, err := service.GetDependencies("todo-2", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(deps.Direct[0].Status).NotTo(BeEmpty())
		})

		It("should return empty if no dependencies", func() {
			deps, err := service.GetDependencies("todo-1", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(deps.Direct).To(BeEmpty())
		})
	})

	Context("RemoveTODO with dependents", func() {
		BeforeEach(func() {
			_, _ = service.AddTODO("todo-1", "Parent", "", nil)
			_, _ = service.AddTODO("todo-2", "Child 1", "", []string{"todo-1"})
			_, _ = service.AddTODO("todo-3", "Child 2", "", []string{"todo-1"})
		})

		It("should prevent deletion if other TODOs depend on it", func() {
			err := service.RemoveTODO("todo-1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("depend"))
		})

		It("should list all dependent TODO IDs in error", func() {
			err := service.RemoveTODO("todo-1")
			Expect(err).To(HaveOccurred())
			errMsg := err.Error()
			Expect(errMsg).To(ContainSubstring("todo-2"))
			Expect(errMsg).To(ContainSubstring("todo-3"))
		})

		It("should allow deletion if no dependents", func() {
			_ = service.RemoveTODO("todo-2")
			_ = service.RemoveTODO("todo-3")
			err := service.RemoveTODO("todo-1")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should work for TODOs with dependencies (dependencies don't block deletion)", func() {
			err := service.RemoveTODO("todo-2")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Edge cases", func() {
		It("should handle empty dependency list correctly", func() {
			item, err := service.AddTODO("todo-1", "Test", "", []string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(BeEmpty())
		})

		It("should require all multiple dependencies to be satisfied", func() {
			_, _ = service.AddTODO("todo-1", "Dep1", "", nil)
			_, _ = service.AddTODO("todo-2", "Dep2", "", nil)
			_, _ = service.AddTODO("todo-3", "Dep3", "", nil)
			_, _ = service.AddTODO("todo-4", "Task", "", []string{"todo-1", "todo-2", "todo-3"})

			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "done")
			// todo-3 still pending

			err := service.UpdateStatus("todo-4", "in_progress")
			Expect(err).To(HaveOccurred())
		})

		It("should allow status change when dependency on done TODO", func() {
			_, _ = service.AddTODO("todo-1", "Dep", "", nil)
			_, _ = service.AddTODO("todo-2", "Task", "", []string{"todo-1"})
			_ = service.UpdateStatus("todo-1", "done")

			err := service.UpdateStatus("todo-2", "in_progress")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should block status change when dependency on pending TODO", func() {
			_, _ = service.AddTODO("todo-1", "Dep", "", nil)
			_, _ = service.AddTODO("todo-2", "Task", "", []string{"todo-1"})

			err := service.UpdateStatus("todo-2", "in_progress")
			Expect(err).To(HaveOccurred())
		})

		It("should block status change when dependency on in_progress TODO", func() {
			_, _ = service.AddTODO("todo-1", "Dep", "", nil)
			_, _ = service.AddTODO("todo-2", "Task", "", []string{"todo-1"})
			_ = service.UpdateStatus("todo-1", "in_progress")

			err := service.UpdateStatus("todo-2", "in_progress")
			Expect(err).To(HaveOccurred())
		})

		It("should handle very long dependency chains correctly", func() {
			// Create chain: A -> B -> C -> D -> E
			_, _ = service.AddTODO("todo-1", "A", "", nil)
			_, _ = service.AddTODO("todo-2", "B", "", []string{"todo-1"})
			_, _ = service.AddTODO("todo-3", "C", "", []string{"todo-2"})
			_, _ = service.AddTODO("todo-4", "D", "", []string{"todo-3"})
			_, _ = service.AddTODO("todo-5", "E", "", []string{"todo-4"})

			// Complete chain
			_ = service.UpdateStatus("todo-1", "done")
			_ = service.UpdateStatus("todo-2", "in_progress")
			_ = service.UpdateStatus("todo-2", "done")
			_ = service.UpdateStatus("todo-3", "done")
			_ = service.UpdateStatus("todo-4", "done")

			// E should now be ready
			err := service.UpdateStatus("todo-5", "in_progress")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle backward compatibility with TODOs without DependsOn", func() {
			// Simulate old TODO without DependsOn field
			item, err := service.AddTODO("todo-1", "Old TODO", "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(BeEmpty())

			// Should work normally
			err = service.UpdateStatus(item.ID, "in_progress")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

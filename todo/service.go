package main

import (
	"fmt"
)

// Service provides business logic for TODO management
type Service struct {
	storage Storage
}

// NewService creates a new Service instance
func NewService(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// findTODOByID finds a TODO item by ID in the list
func (s *Service) findTODOByID(list *TODOList, id string) *TODOItem {
	for i := range list.Items {
		if list.Items[i].ID == id {
			return &list.Items[i]
		}
	}
	return nil
}

// validateDependenciesExist validates that all dependency IDs exist in the list
func (s *Service) validateDependenciesExist(list *TODOList, dependsOn []string) error {
	for _, depID := range dependsOn {
		if s.findTODOByID(list, depID) == nil {
			return fmt.Errorf("dependency TODO with ID '%s' not found", depID)
		}
	}
	return nil
}

// validateIDUniqueness validates that the ID doesn't already exist
func (s *Service) validateIDUniqueness(list *TODOList, id string) error {
	if s.findTODOByID(list, id) != nil {
		return fmt.Errorf("TODO with ID '%s' already exists", id)
	}
	return nil
}

// AddTODO adds a new TODO item
func (s *Service) AddTODO(id, title, assignee string, dependsOn []string) (*TODOItem, error) {
	if id == "" {
		return nil, fmt.Errorf("TODO ID is required")
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	var item *TODOItem
	err := s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		// Validate ID uniqueness
		if err := s.validateIDUniqueness(list, id); err != nil {
			return err
		}

		dependsOnCopy := []string{}
		if dependsOn != nil {
			dependsOnCopy = make([]string, len(dependsOn))
			copy(dependsOnCopy, dependsOn)
			// Validate dependencies exist
			if err := s.validateDependenciesExist(list, dependsOnCopy); err != nil {
				return err
			}
		}

		newItem := TODOItem{
			ID:        id,
			Title:     title,
			Status:    "pending",
			Assignee:  assignee,
			DependsOn: dependsOnCopy,
		}

		list.Items = append(list.Items, newItem)
		item = &newItem

		return s.storage.Save(list)
	})

	if err != nil {
		return nil, err
	}

	return item, nil
}

// checkDependenciesSatisfied checks if all dependencies of a TODO are done
func (s *Service) checkDependenciesSatisfied(list *TODOList, item *TODOItem) (bool, []string) {
	if len(item.DependsOn) == 0 {
		return true, nil
	}

	var blocking []string
	for _, depID := range item.DependsOn {
		dep := s.findTODOByID(list, depID)
		if dep == nil || dep.Status != "done" {
			blocking = append(blocking, depID)
		}
	}

	return len(blocking) == 0, blocking
}

// UpdateStatusWithAgent updates the status of a TODO item with agent permission check
func (s *Service) UpdateStatusWithAgent(id, status, agentName string) error {
	if agentName == "" {
		return fmt.Errorf("agent name is required when not in admin mode")
	}

	validStatuses := map[string]bool{"pending": true, "in_progress": true, "done": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s (must be pending, in_progress, or done)", status)
	}

	return s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		var item *TODOItem
		var currentStatus string
		found := false
		for i := range list.Items {
			if list.Items[i].ID == id {
				item = &list.Items[i]
				currentStatus = item.Status
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("TODO item with ID '%s' not found", id)
		}

		// Check assignee permission
		if item.Assignee == "" {
			return fmt.Errorf("TODO '%s' is not assigned to any agent", id)
		}
		if item.Assignee != agentName {
			return fmt.Errorf("TODO '%s' is not assigned to agent '%s' (assigned to '%s')", id, agentName, item.Assignee)
		}

		// Check dependencies if transitioning to in_progress or done from pending
		if status == "in_progress" && currentStatus == "pending" {
			satisfied, blocking := s.checkDependenciesSatisfied(list, item)
			if !satisfied {
				blockingInfo := []string{}
				for _, blockID := range blocking {
					blockItem := s.findTODOByID(list, blockID)
					if blockItem != nil {
						blockingInfo = append(blockingInfo, fmt.Sprintf("%s (%s)", blockID, blockItem.Status))
					} else {
						blockingInfo = append(blockingInfo, blockID)
					}
				}
				return fmt.Errorf("TODO '%s' is blocked by dependencies: %v", id, blockingInfo)
			}
		} else if status == "done" && currentStatus == "pending" {
			// Allow pending -> done if dependencies are satisfied
			satisfied, blocking := s.checkDependenciesSatisfied(list, item)
			if !satisfied {
				blockingInfo := []string{}
				for _, blockID := range blocking {
					blockItem := s.findTODOByID(list, blockID)
					if blockItem != nil {
						blockingInfo = append(blockingInfo, fmt.Sprintf("%s (%s)", blockID, blockItem.Status))
					} else {
						blockingInfo = append(blockingInfo, blockID)
					}
				}
				return fmt.Errorf("TODO '%s' is blocked by dependencies: %v", id, blockingInfo)
			}
		}
		// Allow in_progress -> done, done -> in_progress, in_progress -> pending without dependency checks

		item.Status = status
		return s.storage.Save(list)
	})
}

// UpdateStatus updates the status of a TODO item (admin/internal use)
func (s *Service) UpdateStatus(id, status string) error {
	validStatuses := map[string]bool{"pending": true, "in_progress": true, "done": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s (must be pending, in_progress, or done)", status)
	}

	return s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		var item *TODOItem
		var currentStatus string
		found := false
		for i := range list.Items {
			if list.Items[i].ID == id {
				item = &list.Items[i]
				currentStatus = item.Status
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("TODO item with ID '%s' not found", id)
		}

		// Check dependencies if transitioning to in_progress or done from pending
		if status == "in_progress" && currentStatus == "pending" {
			satisfied, blocking := s.checkDependenciesSatisfied(list, item)
			if !satisfied {
				blockingInfo := []string{}
				for _, blockID := range blocking {
					blockItem := s.findTODOByID(list, blockID)
					if blockItem != nil {
						blockingInfo = append(blockingInfo, fmt.Sprintf("%s (%s)", blockID, blockItem.Status))
					} else {
						blockingInfo = append(blockingInfo, blockID)
					}
				}
				return fmt.Errorf("TODO '%s' is blocked by dependencies: %v", id, blockingInfo)
			}
		} else if status == "done" && currentStatus == "pending" {
			// Allow pending -> done if dependencies are satisfied
			satisfied, blocking := s.checkDependenciesSatisfied(list, item)
			if !satisfied {
				blockingInfo := []string{}
				for _, blockID := range blocking {
					blockItem := s.findTODOByID(list, blockID)
					if blockItem != nil {
						blockingInfo = append(blockingInfo, fmt.Sprintf("%s (%s)", blockID, blockItem.Status))
					} else {
						blockingInfo = append(blockingInfo, blockID)
					}
				}
				return fmt.Errorf("TODO '%s' is blocked by dependencies: %v", id, blockingInfo)
			}
		}
		// Allow in_progress -> done, done -> in_progress, in_progress -> pending without dependency checks

		item.Status = status
		return s.storage.Save(list)
	})
}

// UpdateAssignee updates the assignee of a TODO item
func (s *Service) UpdateAssignee(id, assignee string) error {
	return s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		found := false
		for i := range list.Items {
			if list.Items[i].ID == id {
				list.Items[i].Assignee = assignee
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("TODO item with ID '%s' not found", id)
		}

		return s.storage.Save(list)
	})
}

// findDependents finds all TODOs that depend on the given TODO ID
func (s *Service) findDependents(list *TODOList, id string) []string {
	var dependents []string
	for _, item := range list.Items {
		for _, depID := range item.DependsOn {
			if depID == id {
				dependents = append(dependents, item.ID)
				break
			}
		}
	}
	return dependents
}

// RemoveTODO removes a TODO item by ID
func (s *Service) RemoveTODO(id string) error {
	return s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		// Check if other TODOs depend on this one
		dependents := s.findDependents(list, id)
		if len(dependents) > 0 {
			return fmt.Errorf("cannot delete TODO '%s': other TODOs depend on it: %v", id, dependents)
		}

		found := false
		newItems := []TODOItem{}
		for _, item := range list.Items {
			if item.ID == id {
				found = true
			} else {
				newItems = append(newItems, item)
			}
		}

		if !found {
			return fmt.Errorf("TODO item with ID '%s' not found", id)
		}

		list.Items = newItems
		return s.storage.Save(list)
	})
}

// ListTODOs returns all TODO items
func (s *Service) ListTODOs() ([]TODOItem, error) {
	var items []TODOItem
	err := s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}
		items = make([]TODOItem, len(list.Items))
		copy(items, list.Items)
		return nil
	})
	return items, err
}

// StatusSummary represents a summary of TODO status
type StatusSummary struct {
	Total      int
	Pending    int
	InProgress int
	Done       int
	Blocked    int
	Ready      int
	ByAssignee map[string]int
}

// GetStatus returns a summary of the TODO list status
func (s *Service) GetStatus() (*StatusSummary, error) {
	var summary *StatusSummary
	err := s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		summary = &StatusSummary{
			Total:      len(list.Items),
			Pending:    0,
			InProgress: 0,
			Done:       0,
			Blocked:    0,
			Ready:      0,
			ByAssignee: make(map[string]int),
		}

		for _, item := range list.Items {
			switch item.Status {
			case "pending":
				summary.Pending++
			case "in_progress":
				summary.InProgress++
			case "done":
				summary.Done++
			}

			if item.Assignee != "" {
				summary.ByAssignee[item.Assignee]++
			}
		}

		return nil
	})
	return summary, err
}

// detectCircularDependency uses DFS to detect if adding a dependency would create a cycle
func (s *Service) detectCircularDependency(list *TODOList, todoID, dependsOnID string) bool {
	// If dependsOnID transitively depends on todoID, adding the dependency would create a cycle
	visited := make(map[string]bool)
	var dfs func(id string) bool
	dfs = func(id string) bool {
		if id == todoID {
			return true // Found cycle
		}
		if visited[id] {
			return false // Already visited this path
		}
		visited[id] = true

		item := s.findTODOByID(list, id)
		if item == nil {
			return false
		}

		for _, depID := range item.DependsOn {
			if dfs(depID) {
				return true
			}
		}
		return false
	}

	return dfs(dependsOnID)
}

// AddDependency adds a dependency to an existing TODO
func (s *Service) AddDependency(todoID, dependsOnID string) error {
	if todoID == dependsOnID {
		return fmt.Errorf("TODO cannot depend on itself")
	}

	return s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		// Find the TODO
		todo := s.findTODOByID(list, todoID)
		if todo == nil {
			return fmt.Errorf("TODO item with ID '%s' not found", todoID)
		}

		// Validate dependency TODO exists
		if s.findTODOByID(list, dependsOnID) == nil {
			return fmt.Errorf("dependency TODO with ID '%s' not found", dependsOnID)
		}

		// Check for circular dependency
		if s.detectCircularDependency(list, todoID, dependsOnID) {
			return fmt.Errorf("adding dependency '%s' to '%s' would create a circular dependency", dependsOnID, todoID)
		}

		// Check for duplicate
		for _, dep := range todo.DependsOn {
			if dep == dependsOnID {
				return fmt.Errorf("dependency '%s' already exists for TODO '%s'", dependsOnID, todoID)
			}
		}

		// Add dependency
		todo.DependsOn = append(todo.DependsOn, dependsOnID)

		return s.storage.Save(list)
	})
}

// GetReadyTODOs returns TODOs that are pending with all dependencies satisfied
func (s *Service) GetReadyTODOs() ([]TODOItem, error) {
	var ready []TODOItem
	err := s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		for _, item := range list.Items {
			if item.Status == "pending" {
				satisfied, _ := s.checkDependenciesSatisfied(list, &item)
				if satisfied {
					ready = append(ready, item)
				}
			}
		}
		return nil
	})
	return ready, err
}

// GetBlockedTODOs returns TODOs that are pending with unsatisfied dependencies
func (s *Service) GetBlockedTODOs() ([]BlockedTODO, error) {
	var blocked []BlockedTODO
	err := s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		for _, item := range list.Items {
			if item.Status == "pending" && len(item.DependsOn) > 0 {
				satisfied, blockingIDs := s.checkDependenciesSatisfied(list, &item)
				if !satisfied {
					blockingInfo := []BlockingInfo{}
					for _, blockID := range blockingIDs {
						blockItem := s.findTODOByID(list, blockID)
						if blockItem != nil {
							blockingInfo = append(blockingInfo, BlockingInfo{
								ID:     blockItem.ID,
								Title:  blockItem.Title,
								Status: blockItem.Status,
							})
						}
					}
					blocked = append(blocked, BlockedTODO{
						ID:        item.ID,
						Title:     item.Title,
						Status:    item.Status,
						Assignee:  item.Assignee,
						BlockedBy: blockingInfo,
					})
				}
			}
		}
		return nil
	})
	return blocked, err
}

// GetDependencies returns dependencies for a TODO
func (s *Service) GetDependencies(id string, transitive bool) (*GetTODODependenciesOutput, error) {
	var result *GetTODODependenciesOutput
	err := s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		item := s.findTODOByID(list, id)
		if item == nil {
			return fmt.Errorf("TODO item with ID '%s' not found", id)
		}

		result = &GetTODODependenciesOutput{
			Direct:      []DependencyInfo{},
			Transitive:  []DependencyInfo{},
			DirectCount: len(item.DependsOn),
		}

		// Get direct dependencies
		directMap := make(map[string]bool)
		for _, depID := range item.DependsOn {
			dep := s.findTODOByID(list, depID)
			if dep != nil {
				result.Direct = append(result.Direct, DependencyInfo{
					ID:     dep.ID,
					Title:  dep.Title,
					Status: dep.Status,
				})
				directMap[depID] = true
			}
		}

		// Get transitive dependencies if requested
		if transitive {
			visited := make(map[string]bool)
			var collectTransitive func(depID string)
			collectTransitive = func(depID string) {
				if visited[depID] || directMap[depID] {
					return
				}
				visited[depID] = true
				dep := s.findTODOByID(list, depID)
				if dep != nil {
					result.Transitive = append(result.Transitive, DependencyInfo{
						ID:     dep.ID,
						Title:  dep.Title,
						Status: dep.Status,
					})
					for _, subDepID := range dep.DependsOn {
						collectTransitive(subDepID)
					}
				}
			}

			for _, depID := range item.DependsOn {
				dep := s.findTODOByID(list, depID)
				if dep != nil {
					for _, subDepID := range dep.DependsOn {
						collectTransitive(subDepID)
					}
				}
			}
		}

		return nil
	})
	return result, err
}

// RemoveDependency removes a dependency from a TODO
func (s *Service) RemoveDependency(todoID, dependsOnID string) error {
	return s.storage.WithLock(func() error {
		list, err := s.storage.Load()
		if err != nil {
			return err
		}

		// Find the TODO
		todo := s.findTODOByID(list, todoID)
		if todo == nil {
			return fmt.Errorf("TODO item with ID '%s' not found", todoID)
		}

		// Remove dependency if it exists
		newDeps := []string{}
		for _, dep := range todo.DependsOn {
			if dep != dependsOnID {
				newDeps = append(newDeps, dep)
			}
		}

		todo.DependsOn = newDeps

		return s.storage.Save(list)
	})
}

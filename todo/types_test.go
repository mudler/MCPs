package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Types", func() {
	Context("TODOItem JSON serialization", func() {
		It("should serialize TODOItem with DependsOn correctly", func() {
			item := TODOItem{
				ID:        "todo-1",
				Title:     "Test TODO",
				Status:    "pending",
				Assignee:  "agent1",
				DependsOn: []string{"todo-2", "todo-3"},
			}

			data, err := json.Marshal(item)
			Expect(err).NotTo(HaveOccurred())

			var result TODOItem
			err = json.Unmarshal(data, &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.DependsOn).To(Equal([]string{"todo-2", "todo-3"}))
		})

		It("should omit DependsOn field when empty in JSON", func() {
			item := TODOItem{
				ID:        "todo-1",
				Title:     "Test TODO",
				Status:    "pending",
				Assignee:  "agent1",
				DependsOn: []string{},
			}

			data, err := json.Marshal(item)
			Expect(err).NotTo(HaveOccurred())

			// Check that depends_on is omitted when empty (due to omitempty tag)
			var jsonMap map[string]interface{}
			err = json.Unmarshal(data, &jsonMap)
			Expect(err).NotTo(HaveOccurred())
			// When empty, it should be omitted from JSON
			Expect(jsonMap).NotTo(HaveKey("depends_on"))
		})

		It("should deserialize TODOItem from JSON with depends_on field", func() {
			jsonData := `{"id":"todo-1","title":"Test","status":"pending","assignee":"agent1","depends_on":["todo-2"]}`

			var item TODOItem
			err := json.Unmarshal([]byte(jsonData), &item)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(Equal([]string{"todo-2"}))
		})

		It("should deserialize TODOItem from JSON without depends_on field (backward compatible)", func() {
			jsonData := `{"id":"todo-1","title":"Test","status":"pending","assignee":"agent1"}`

			var item TODOItem
			err := json.Unmarshal([]byte(jsonData), &item)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(BeEmpty())
		})

		It("should handle nil DependsOn as empty slice", func() {
			jsonData := `{"id":"todo-1","title":"Test","status":"pending","depends_on":null}`

			var item TODOItem
			err := json.Unmarshal([]byte(jsonData), &item)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.DependsOn).To(BeEmpty())
		})
	})

	Context("TODOItem validation", func() {
		It("should allow empty DependsOn slice", func() {
			item := TODOItem{
				ID:        "todo-1",
				Title:     "Test",
				Status:    "pending",
				DependsOn: []string{},
			}
			Expect(item.DependsOn).To(BeEmpty())
		})

		It("should allow DependsOn with duplicate IDs (validation at service level)", func() {
			item := TODOItem{
				ID:        "todo-1",
				Title:     "Test",
				Status:    "pending",
				DependsOn: []string{"todo-2", "todo-2"},
			}
			Expect(item.DependsOn).To(HaveLen(2))
		})
	})
})

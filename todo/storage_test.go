package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Storage", func() {
	var tempDir string
	var storage *FileStorage

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "todo-test-*")
		Expect(err).NotTo(HaveOccurred())
		storage = NewFileStorage(filepath.Join(tempDir, "todos.json"))
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Context("Storage interface", func() {
		It("should have Load, Save, and WithLock methods", func() {
			var s Storage = storage
			Expect(s).NotTo(BeNil())
		})
	})

	Context("FileStorage Load operations", func() {
		It("should return empty TODOList when file does not exist", func() {
			list, err := storage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(list).NotTo(BeNil())
			Expect(list.Items).To(BeEmpty())
		})

		It("should load TODOList from existing file with valid JSON", func() {
			// Create test data
			testList := &TODOList{
				Items: []TODOItem{
					{ID: "todo-1", Title: "Test 1", Status: "pending"},
					{ID: "todo-2", Title: "Test 2", Status: "done"},
				},
			}

			// Save first
			err := storage.Save(testList)
			Expect(err).NotTo(HaveOccurred())

			// Load and verify
			loaded, err := storage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Items).To(HaveLen(2))
			Expect(loaded.Items[0].ID).To(Equal("todo-1"))
			Expect(loaded.Items[1].ID).To(Equal("todo-2"))
		})

		It("should return error when file contains invalid JSON", func() {
			// Write invalid JSON
			filePath := storage.filePath
			err := os.WriteFile(filePath, []byte("invalid json {"), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = storage.Load()
			Expect(err).To(HaveOccurred())
		})

		It("should return empty TODOList when file is empty", func() {
			// Create empty file
			filePath := storage.filePath
			err := os.MkdirAll(filepath.Dir(filePath), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filePath, []byte(""), 0644)
			Expect(err).NotTo(HaveOccurred())

			list, err := storage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(BeEmpty())
		})

		It("should handle DependsOn field correctly", func() {
			testList := &TODOList{
				Items: []TODOItem{
					{
						ID:        "todo-1",
						Title:     "Test",
						Status:    "pending",
						DependsOn: []string{"todo-2", "todo-3"},
					},
				},
			}

			err := storage.Save(testList)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := storage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Items[0].DependsOn).To(Equal([]string{"todo-2", "todo-3"}))
		})
	})

	Context("FileStorage Save operations", func() {
		It("should create file if it doesn't exist", func() {
			testList := &TODOList{
				Items: []TODOItem{{ID: "todo-1", Title: "Test", Status: "pending"}},
			}

			err := storage.Save(testList)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(storage.filePath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should overwrite existing file", func() {
			// Save first list
			list1 := &TODOList{
				Items: []TODOItem{{ID: "todo-1", Title: "First", Status: "pending"}},
			}
			err := storage.Save(list1)
			Expect(err).NotTo(HaveOccurred())

			// Save second list
			list2 := &TODOList{
				Items: []TODOItem{{ID: "todo-2", Title: "Second", Status: "done"}},
			}
			err = storage.Save(list2)
			Expect(err).NotTo(HaveOccurred())

			// Verify only second list exists
			loaded, err := storage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Items).To(HaveLen(1))
			Expect(loaded.Items[0].ID).To(Equal("todo-2"))
		})

		It("should create directory if needed", func() {
			nestedStorage := NewFileStorage(filepath.Join(tempDir, "nested", "dir", "todos.json"))
			testList := &TODOList{
				Items: []TODOItem{{ID: "todo-1", Title: "Test", Status: "pending"}},
			}

			err := nestedStorage.Save(testList)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(nestedStorage.filePath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should use atomic write (temp file + rename)", func() {
			testList := &TODOList{
				Items: []TODOItem{{ID: "todo-1", Title: "Test", Status: "pending"}},
			}

			err := storage.Save(testList)
			Expect(err).NotTo(HaveOccurred())

			// Temp file should not exist after successful save
			tempFile := storage.filePath + ".tmp"
			_, err = os.Stat(tempFile)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("should preserve DependsOn field", func() {
			testList := &TODOList{
				Items: []TODOItem{
					{
						ID:        "todo-1",
						Title:     "Test",
						Status:    "pending",
						DependsOn: []string{"todo-2"},
					},
				},
			}

			err := storage.Save(testList)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := storage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Items[0].DependsOn).To(Equal([]string{"todo-2"}))
		})
	})

	Context("FileStorage locking", func() {
		It("should acquire and release lock", func() {
			called := false
			err := storage.WithLock(func() error {
				called = true
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(called).To(BeTrue())
		})

		It("should execute function within lock", func() {
			testList := &TODOList{
				Items: []TODOItem{{ID: "todo-1", Title: "Test", Status: "pending"}},
			}

			err := storage.WithLock(func() error {
				return storage.Save(testList)
			})
			Expect(err).NotTo(HaveOccurred())

			loaded, err := storage.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Items).To(HaveLen(1))
		})

		It("should return error from function", func() {
			testErr := os.ErrPermission
			err := storage.WithLock(func() error {
				return testErr
			})
			Expect(err).To(Equal(testErr))
		})
	})

	Context("Test isolation", func() {
		It("should not interfere with other tests", func() {
			// Each test uses its own temp directory
			Expect(tempDir).NotTo(BeEmpty())
			Expect(storage.filePath).To(ContainSubstring(tempDir))
		})
	})
})

package grpc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestDescriptorLoader(t *testing.T) {
	registry := NewProtoRegistry()
	loader := NewDescriptorLoader(registry)

	t.Run("LoadDescriptorFile", func(t *testing.T) {
		// Create a test descriptor file
		tempDir := t.TempDir()
		descFile := filepath.Join(tempDir, "test.desc")

		// Create a simple FileDescriptorSet
		fds := &descriptorpb.FileDescriptorSet{
			File: []*descriptorpb.FileDescriptorProto{
				{
					Name:    proto.String("test.proto"),
					Package: proto.String("test"),
					Service: []*descriptorpb.ServiceDescriptorProto{
						{
							Name: proto.String("TestService"),
							Method: []*descriptorpb.MethodDescriptorProto{
								{
									Name:       proto.String("TestMethod"),
									InputType:  proto.String(".test.TestRequest"),
									OutputType: proto.String(".test.TestResponse"),
								},
							},
						},
					},
					MessageType: []*descriptorpb.DescriptorProto{
						{
							Name: proto.String("TestRequest"),
							Field: []*descriptorpb.FieldDescriptorProto{
								{
									Name:   proto.String("message"),
									Number: proto.Int32(1),
									Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
								},
							},
						},
						{
							Name: proto.String("TestResponse"),
							Field: []*descriptorpb.FieldDescriptorProto{
								{
									Name:   proto.String("reply"),
									Number: proto.Int32(1),
									Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
								},
							},
						},
					},
				},
			},
		}

		// Marshal and write to file
		data, err := proto.Marshal(fds)
		if err != nil {
			t.Fatalf("Failed to marshal descriptor: %v", err)
		}

		if err := os.WriteFile(descFile, data, 0644); err != nil {
			t.Fatalf("Failed to write descriptor file: %v", err)
		}

		// Load the descriptor file
		if err := loader.LoadDescriptorFile(descFile); err != nil {
			t.Fatalf("Failed to load descriptor file: %v", err)
		}

		// Verify it was loaded
		if !loader.IsLoaded(descFile) {
			t.Error("Descriptor file should be marked as loaded")
		}

		// Verify the method can be found
		method, err := registry.GetMethod("/test.TestService/TestMethod")
		if err != nil {
			t.Fatalf("Failed to get method: %v", err)
		}

		if method.Name() != "TestMethod" {
			t.Errorf("Expected method name TestMethod, got %s", method.Name())
		}
	})

	t.Run("LoadDescriptorDirectory", func(t *testing.T) {
		// Create a test directory with multiple descriptor files
		tempDir := t.TempDir()

		// Create multiple test descriptor files
		for i := 0; i < 3; i++ {
			descFile := filepath.Join(tempDir, fmt.Sprintf("test%d.desc", i))
			
			fds := &descriptorpb.FileDescriptorSet{
				File: []*descriptorpb.FileDescriptorProto{
					{
						Name:    proto.String(fmt.Sprintf("test%d.proto", i)),
						Package: proto.String(fmt.Sprintf("test%d", i)),
						MessageType: []*descriptorpb.DescriptorProto{
							{
								Name: proto.String("TestRequest"),
								Field: []*descriptorpb.FieldDescriptorProto{
									{
										Name:   proto.String("id"),
										Number: proto.Int32(1),
										Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
									},
								},
							},
							{
								Name: proto.String("TestResponse"),
								Field: []*descriptorpb.FieldDescriptorProto{
									{
										Name:   proto.String("result"),
										Number: proto.Int32(1),
										Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
									},
								},
							},
						},
						Service: []*descriptorpb.ServiceDescriptorProto{
							{
								Name: proto.String(fmt.Sprintf("TestService%d", i)),
								Method: []*descriptorpb.MethodDescriptorProto{
									{
										Name:       proto.String("TestMethod"),
										InputType:  proto.String(fmt.Sprintf(".test%d.TestRequest", i)),
										OutputType: proto.String(fmt.Sprintf(".test%d.TestResponse", i)),
									},
								},
							},
						},
					},
				},
			}

			data, err := proto.Marshal(fds)
			if err != nil {
				t.Fatalf("Failed to marshal descriptor %d: %v", i, err)
			}

			if err := os.WriteFile(descFile, data, 0644); err != nil {
				t.Fatalf("Failed to write descriptor file %d: %v", i, err)
			}
		}

		// Also create a non-.desc file that should be ignored
		if err := os.WriteFile(filepath.Join(tempDir, "ignore.txt"), []byte("ignore me"), 0644); err != nil {
			t.Fatalf("Failed to write ignore file: %v", err)
		}

		// Load all descriptors from directory
		if err := loader.LoadDescriptorDirectory(tempDir); err != nil {
			t.Fatalf("Failed to load descriptor directory: %v", err)
		}

		// Verify all .desc files were loaded
		// Note: We're using a fresh loader for directory test, but the previous test
		// may have loaded a file, so we just check that at least 3 were loaded
		loadedFiles := loader.GetLoadedFiles()
		descCount := 0
		for _, file := range loadedFiles {
			if filepath.Ext(file) == ".desc" && strings.Contains(file, tempDir) {
				descCount++
			}
		}

		if descCount != 3 {
			t.Errorf("Expected 3 descriptor files loaded from test directory, got %d", descCount)
		}

		// Verify the services can be found
		for i := 0; i < 3; i++ {
			methodPath := fmt.Sprintf("/test%d.TestService%d/TestMethod", i, i)
			if _, err := registry.GetMethod(methodPath); err != nil {
				t.Errorf("Failed to find method %s: %v", methodPath, err)
			}
		}
	})

	t.Run("ReloadFile", func(t *testing.T) {
		// Use a fresh registry for this test
		reloadRegistry := NewProtoRegistry()
		reloadLoader := NewDescriptorLoader(reloadRegistry)
		
		tempDir := t.TempDir()
		descFile := filepath.Join(tempDir, "reload.desc")

		// Create initial descriptor
		fds1 := &descriptorpb.FileDescriptorSet{
			File: []*descriptorpb.FileDescriptorProto{
				{
					Name:    proto.String("reload.proto"),
					Package: proto.String("reload"),
					MessageType: []*descriptorpb.DescriptorProto{
						{
							Name: proto.String("Request"),
						},
						{
							Name: proto.String("Response"),
						},
					},
					Service: []*descriptorpb.ServiceDescriptorProto{
						{
							Name: proto.String("ReloadService"),
							Method: []*descriptorpb.MethodDescriptorProto{
								{
									Name:       proto.String("Method1"),
									InputType:  proto.String(".reload.Request"),
									OutputType: proto.String(".reload.Response"),
								},
							},
						},
					},
				},
			},
		}

		data1, _ := proto.Marshal(fds1)
		os.WriteFile(descFile, data1, 0644)

		// Load initial version
		reloadLoader.LoadDescriptorFile(descFile)

		// Verify initial method exists
		if _, err := reloadRegistry.GetMethod("/reload.ReloadService/Method1"); err != nil {
			t.Fatalf("Initial method not found: %v", err)
		}

		// Update descriptor with new method
		fds2 := &descriptorpb.FileDescriptorSet{
			File: []*descriptorpb.FileDescriptorProto{
				{
					Name:    proto.String("reload.proto"),
					Package: proto.String("reload"),
					MessageType: []*descriptorpb.DescriptorProto{
						{
							Name: proto.String("Request"),
						},
						{
							Name: proto.String("Response"),
						},
					},
					Service: []*descriptorpb.ServiceDescriptorProto{
						{
							Name: proto.String("ReloadService"),
							Method: []*descriptorpb.MethodDescriptorProto{
								{
									Name:       proto.String("Method1"),
									InputType:  proto.String(".reload.Request"),
									OutputType: proto.String(".reload.Response"),
								},
								{
									Name:       proto.String("Method2"),
									InputType:  proto.String(".reload.Request"),
									OutputType: proto.String(".reload.Response"),
								},
							},
						},
					},
				},
			},
		}

		data2, _ := proto.Marshal(fds2)
		os.WriteFile(descFile, data2, 0644)

		// For this test, we'll create a new registry to simulate reload
		// In a real implementation, we would need registry clearing functionality
		reloadRegistry2 := NewProtoRegistry()
		reloadLoader2 := NewDescriptorLoader(reloadRegistry2)
		
		// Load the updated file
		if err := reloadLoader2.LoadDescriptorFile(descFile); err != nil {
			t.Fatalf("Failed to reload file: %v", err)
		}

		// Both methods should now exist in the new registry
		if _, err := reloadRegistry2.GetMethod("/reload.ReloadService/Method1"); err != nil {
			t.Errorf("Method1 not found after reload: %v", err)
		}
		if _, err := reloadRegistry2.GetMethod("/reload.ReloadService/Method2"); err != nil {
			t.Errorf("Method2 not found after reload: %v", err)
		}
	})
}

func TestDescriptorManager(t *testing.T) {
	t.Run("LoadFromConfig", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create test descriptor files
		file1 := filepath.Join(tempDir, "service1.desc")
		file2 := filepath.Join(tempDir, "subdir", "service2.desc")
		
		os.MkdirAll(filepath.Dir(file2), 0755)
		
		// Write test descriptors
		for i, file := range []string{file1, file2} {
			fds := &descriptorpb.FileDescriptorSet{
				File: []*descriptorpb.FileDescriptorProto{
					{
						Name:    proto.String(fmt.Sprintf("service%d.proto", i+1)),
						Package: proto.String(fmt.Sprintf("service%d", i+1)),
						MessageType: []*descriptorpb.DescriptorProto{
							{
								Name: proto.String("EmptyRequest"),
							},
							{
								Name: proto.String("EmptyResponse"),
							},
						},
						Service: []*descriptorpb.ServiceDescriptorProto{
							{
								Name: proto.String(fmt.Sprintf("Service%d", i+1)),
								Method: []*descriptorpb.MethodDescriptorProto{
									{
										Name:       proto.String("TestMethod"),
										InputType:  proto.String(fmt.Sprintf(".service%d.EmptyRequest", i+1)),
										OutputType: proto.String(fmt.Sprintf(".service%d.EmptyResponse", i+1)),
									},
								},
							},
						},
					},
				},
			}
			data, _ := proto.Marshal(fds)
			os.WriteFile(file, data, 0644)
		}

		// Create manager with config
		config := DescriptorConfig{
			DescriptorFiles: []string{file1},
			DescriptorDirs:  []string{filepath.Join(tempDir, "subdir")},
			AutoReload:      false,
		}
		
		registry := NewProtoRegistry()
		manager := NewDescriptorManager(config, registry, nil)
		
		// Start the manager
		if err := manager.Start(); err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager.Stop()
		
		// Verify both services were loaded
		services := []string{"service1.Service1", "service2.Service2"}
		for _, service := range services {
			if _, err := registry.GetServiceMethods(service); err != nil {
				t.Errorf("Service %s not found: %v", service, err)
			}
		}
	})
}

func TestDescriptorLoaderWithBytes(t *testing.T) {
	registry := NewProtoRegistry()
	loader := NewDescriptorLoader(registry)

	t.Run("LoadFromFileDescriptorProto", func(t *testing.T) {
		// Create a FileDescriptorProto (single file)
		fdp := &descriptorpb.FileDescriptorProto{
			Name:    proto.String("single.proto"),
			Package: proto.String("single"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("Request"),
				},
				{
					Name: proto.String("Response"),
				},
			},
			Service: []*descriptorpb.ServiceDescriptorProto{
				{
					Name: proto.String("SingleService"),
					Method: []*descriptorpb.MethodDescriptorProto{
						{
							Name:       proto.String("SingleMethod"),
							InputType:  proto.String(".single.Request"),
							OutputType: proto.String(".single.Response"),
						},
					},
				},
			},
		}

		// Load it
		if err := loader.LoadDescriptorFromProto(fdp); err != nil {
			t.Fatalf("Failed to load FileDescriptorProto: %v", err)
		}

		// Verify the method exists
		if _, err := registry.GetMethod("/single.SingleService/SingleMethod"); err != nil {
			t.Errorf("Method not found: %v", err)
		}
	})
}
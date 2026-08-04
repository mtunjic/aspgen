package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedLayoutIntegration exercises the public generation workflow and
// checks that each profile puts files in the project that owns them. This is
// intentionally filesystem-based rather than template-based: a template can
// render successfully while still targeting the wrong layer or project.
func TestGeneratedLayoutIntegration(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		entity       []string
		want         []string
		forbid       []string
		inSolution   []string
		projectRoots []string
	}{
		{
			name:         "clean webapi",
			args:         []string{"new", "CleanApi", "--app", "webapi"},
			entity:       []string{"Person", "name:string", "age:int", "born:date", "active:bool"},
			want:         []string{"src/Domain/Entities/Person.cs"},
			forbid:       []string{"src/WebApi/Models/Person.cs", "src/WebApi/Data/AppDbContext.cs", "src/Infrastructure/Persistence/PersonRepository.cs"},
			inSolution:   []string{"src\\Domain\\Domain.csproj", "src\\Application\\Application.csproj", "src\\Infrastructure\\Infrastructure.csproj", "src\\WebApi\\WebApi.csproj"},
			projectRoots: []string{"src/Domain", "src/Application", "src/Infrastructure", "src/WebApi"},
		},
		{
			name:         "ddd fullstack",
			args:         []string{"new", "DddApp", "--app", "fullstack", "--backend:ddd"},
			entity:       []string{"Person", "name:string", "age:int"},
			want:         []string{"src/Domain/Entities/Person.cs", "src/Infrastructure/Persistence/PersonRepository.cs", "src/Application/Features/Person/CreatePersonCommand.cs", "src/WebApi/Features/Person/PersonEndpoints.cs", "src/Desktop/Modules/Person/PersonModule.cs"},
			forbid:       []string{"src/WebApi/Models/Person.cs", "src/WebApi/Data/AppDbContext.cs"},
			inSolution:   []string{"src\\Domain\\Domain.csproj", "src\\Application\\Application.csproj", "src\\Infrastructure\\Infrastructure.csproj", "src\\WebApi\\WebApi.csproj", "src\\Desktop\\DddApp.Desktop.csproj"},
			projectRoots: []string{"src/Domain", "src/Application", "src/Infrastructure", "src/WebApi", "src/Desktop"},
		},
		{
			name:   "simple fullstack",
			args:   []string{"new", "SimpleApp", "--app", "fullstack", "--simple", "--theme:wpfui"},
			entity: []string{"Person", "name:string", "age:int", "born:date", "active:bool"},
			want: []string{
				"src/WebApi/Models/Person.cs",
				"src/WebApi/Data/AppDbContext.cs",
				"src/WebApi/Features/Person/PersonEndpoints.cs",
				"src/Desktop/Modules/Person/PersonModule.cs",
				"src/Desktop/Modules/Person/Views/PersonView.xaml",
			},
			forbid: []string{
				"src/Domain/Entities/Person.cs",
				"src/Application/Features/Person/CreatePersonCommand.cs",
				"src/Infrastructure/Persistence/PersonRepository.cs",
			},
			inSolution:   []string{"src\\WebApi\\WebApi.csproj", "src\\Desktop\\SimpleApp.Desktop.csproj"},
			projectRoots: []string{"src/WebApi", "src/Desktop"},
		},
		{
			name:         "wpf only",
			args:         []string{"new", "DesktopApp", "--app", "wpf"},
			entity:       []string{"Person", "name:string", "active:bool"},
			want:         []string{"src/Desktop/Modules/Person/PersonModule.cs", "src/Desktop/Modules/Person/Services/PersonStore.cs"},
			forbid:       []string{"src/WebApi/Features/Person/PersonEndpoints.cs", "src/Domain/Entities/Person.cs"},
			inSolution:   []string{"src\\Desktop\\DesktopApp.Desktop.csproj"},
			projectRoots: []string{"src/Desktop"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), filepath.Base(test.args[1]))
			args := append(append([]string{}, test.args...), "--output", project)
			if err := Run(args); err != nil {
				t.Fatal(err)
			}
			if err := Run(append(append([]string{"add", "entity"}, test.entity...), "--project", project)); err != nil {
				t.Fatal(err)
			}

			for _, relative := range test.want {
				assertExists(t, project, relative)
			}
			for _, relative := range test.forbid {
				if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(relative))); err == nil {
					t.Errorf("file is in the wrong generated location: %s", relative)
				}
			}

			manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest Manifest
			if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
				t.Fatal(err)
			}
			if !contains(manifest.Components, "entity:"+test.entity[0]) {
				t.Fatalf("manifest does not record generated entity: %#v", manifest.Components)
			}

			solution, err := os.ReadFile(filepath.Join(project, manifest.Project+".sln"))
			if err != nil {
				t.Fatal(err)
			}
			for _, projectPath := range test.inSolution {
				if !strings.Contains(string(solution), projectPath) {
					t.Errorf("solution is missing project %s", projectPath)
				}
			}
			assertNoOrphanSourceFiles(t, project, test.projectRoots)
		})
	}
}

func assertNoOrphanSourceFiles(t *testing.T, root string, projectRoots []string) {
	t.Helper()
	known := make(map[string]bool, len(projectRoots))
	for _, projectRoot := range projectRoots {
		known[filepath.Clean(filepath.FromSlash(projectRoot))] = true
	}
	err := filepath.Walk(filepath.Join(root, "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".cs" && extension != ".xaml" && extension != ".csproj" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) < 2 || !known[filepath.Clean(filepath.FromSlash(parts[0]+"/"+parts[1]))] {
			t.Errorf("source file is outside every generated project: %s", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertExists(t *testing.T, root, relative string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
		t.Errorf("expected generated file %s: %v", relative, err)
	}
}

func TestIncrementalPlacementGuards(t *testing.T) {
	api := filepath.Join(t.TempDir(), "Api")
	if err := Run([]string{"new", "Api", "--app", "webapi", "--output", api}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "--theme:wpfui", "--project", api}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "module", "Reports", "--project", api}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, api, "src/Desktop/Api.Desktop.csproj")
	assertExists(t, api, "src/Desktop/Modules/Reports/ReportsModule.cs")
	solution, err := os.ReadFile(filepath.Join(api, "Api.sln"))
	if err != nil || !strings.Contains(string(solution), `src\Desktop\Api.Desktop.csproj`) {
		t.Fatalf("incremental UI was not added to the solution: %v", err)
	}

	simple := filepath.Join(t.TempDir(), "Simple")
	if err := Run([]string{"new", "Simple", "--app", "webapi", "--simple", "--output", simple}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "database", "postgres", "--project", simple}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(simple, "src", "Infrastructure")); !os.IsNotExist(err) {
		t.Fatalf("simple database addition created an unrelated Infrastructure directory: %v", err)
	}

	plain := filepath.Join(t.TempDir(), "Plain")
	if err := Run([]string{"new", "Plain", "--app", "webapi", "--output", plain}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "module", "Reports", "--project", plain}); err == nil {
		t.Fatal("expected module generation to reject a project without WPF")
	}
	if _, err := os.Stat(filepath.Join(plain, "src", "Desktop")); !os.IsNotExist(err) {
		t.Fatalf("rejected module generation left Desktop files behind: %v", err)
	}
	desktopOnly := filepath.Join(t.TempDir(), "DesktopOnly")
	if err := Run([]string{"new", "DesktopOnly", "--app", "wpf", "--output", desktopOnly}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "database", "postgres", "--project", desktopOnly}); err == nil {
		t.Fatal("expected database generation to reject a project without Web API")
	}
	if _, err := os.Stat(filepath.Join(desktopOnly, "src", "Infrastructure")); !os.IsNotExist(err) {
		t.Fatalf("rejected database generation left Infrastructure files behind: %v", err)
	}

	if err := Run([]string{"add", "feature", "CreatePerson", "name:string", "--project", simple}); err == nil {
		t.Fatal("expected feature generation to reject a simple project")
	}
	if err := Run([]string{"add", "service", "Email", "--project", simple}); err == nil {
		t.Fatal("expected service generation to reject a simple project")
	}
	if err := Run([]string{"add", "entity", "Order", "total:decimal", "--backend:ddd", "--project", simple}); err == nil {
		t.Fatal("expected conflicting backend generation to be rejected")
	}
	if _, err := os.Stat(filepath.Join(simple, "src", "Application")); !os.IsNotExist(err) {
		t.Fatalf("rejected simple operations left Application files behind: %v", err)
	}
}

func TestEntityRelationshipGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "RelDemo")
	if err := Run([]string{"new", "RelDemo", "--app", "fullstack", "--simple", "--theme:wpfui", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Customer", "name:string", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Order", "total:decimal", "customer:Customer", "notes:string?", "--project", project}); err != nil {
		t.Fatal(err)
	}

	model, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Models", "Order.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(model), "public long CustomerId { get; set; }") {
		t.Fatalf("expected synthesized foreign key property in Order model: %s", model)
	}
	if !strings.Contains(string(model), "public Customer? Customer { get; set; }") {
		t.Fatalf("expected navigation property in Order model: %s", model)
	}

	dbContext, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dbContext), "modelBuilder.Entity<Order>().HasOne(x => x.Customer).WithMany().HasForeignKey(x => x.CustomerId);") {
		t.Fatalf("expected relation fluent config in AppDbContext.cs: %s", dbContext)
	}

	view, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Order", "Views", "OrderView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(view), `ItemsSource="{Binding CustomerItems}"`) || !strings.Contains(string(view), `DisplayMemberPath="Name"`) {
		t.Fatalf("expected a Customer picker ComboBox in OrderView.xaml: %s", view)
	}

	viewModel, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Order", "ViewModels", "OrderViewModel.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(viewModel), "ICustomerStore customerStore") || !strings.Contains(string(viewModel), "ObservableCollection<CustomerRow> CustomerItems") {
		t.Fatalf("expected Customer store injection and item collection in OrderViewModel.cs: %s", viewModel)
	}

	customerModel, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Models", "Customer.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customerModel), "public ICollection<Order> Orders { get; set; } = [];") {
		t.Fatalf("expected inverse Orders navigation in Customer model: %s", customerModel)
	}

	customerView, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Customer", "Views", "CustomerView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customerView), `ItemsSource="{Binding Orders}"`) {
		t.Fatalf("expected a read-only Orders grid in CustomerView.xaml: %s", customerView)
	}

	customerViewModel, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Customer", "ViewModels", "CustomerViewModel.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customerViewModel), "IOrderStore orderStore") || !strings.Contains(string(customerViewModel), "ObservableCollection<OrderRow> Orders") {
		t.Fatalf("expected an injected IOrderStore and Orders collection in CustomerViewModel.cs: %s", customerViewModel)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entities) != 2 {
		t.Fatalf("expected two recorded entities, got %#v", manifest.Entities)
	}

	if err := Run([]string{"add", "entity", "Invoice", "amount:decimal", "vendor:Unknown", "--project", project}); err == nil {
		t.Fatal("expected an error when referencing an unknown relation target")
	}
	if _, err := os.Stat(filepath.Join(project, "src", "WebApi", "Models", "Invoice.cs")); !os.IsNotExist(err) {
		t.Fatalf("rejected entity generation left files behind: %v", err)
	}
}

func TestManyToManyRelationGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "M2MDemo")
	if err := Run([]string{"new", "M2MDemo", "--app", "fullstack", "--simple", "--theme:wpfui", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Tag", "name:string", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Post", "title:string", "tags:Tag[]", "--project", project}); err != nil {
		t.Fatal(err)
	}

	joinModel, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Models", "PostTag.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(joinModel), "public long PostId { get; set; }") || !strings.Contains(string(joinModel), "public long TagId { get; set; }") {
		t.Fatalf("expected both foreign keys in the PostTag join model: %s", joinModel)
	}

	dbContext, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dbContext), "public DbSet<PostTag> PostTags => Set<PostTag>();") {
		t.Fatalf("expected a PostTag DbSet in AppDbContext.cs: %s", dbContext)
	}
	if !strings.Contains(string(dbContext), "modelBuilder.Entity<PostTag>().HasOne(x => x.Post).WithMany().HasForeignKey(x => x.PostId);") ||
		!strings.Contains(string(dbContext), "modelBuilder.Entity<PostTag>().HasOne(x => x.Tag).WithMany().HasForeignKey(x => x.TagId);") {
		t.Fatalf("expected both fluent relation configs for PostTag in AppDbContext.cs: %s", dbContext)
	}

	postModel, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Models", "Post.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(postModel), "public ICollection<PostTag> PostTags { get; set; } = [];") {
		t.Fatalf("expected inverse PostTags navigation on Post: %s", postModel)
	}

	tagModel, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Models", "Tag.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tagModel), "public ICollection<PostTag> PostTags { get; set; } = [];") {
		t.Fatalf("expected inverse PostTags navigation on Tag: %s", tagModel)
	}

	joinView, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "PostTag", "Views", "PostTagView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(joinView), `ItemsSource="{Binding PostItems}"`) || !strings.Contains(string(joinView), `ItemsSource="{Binding TagItems}"`) {
		t.Fatalf("expected Post and Tag picker ComboBoxes in PostTagView.xaml: %s", joinView)
	}

	// re-running with --force must be idempotent: no duplicate markers/entries.
	if err := Run([]string{"add", "entity", "Post", "title:string", "tags:Tag[]", "--project", project, "--force"}); err != nil {
		t.Fatal(err)
	}
	dbContextAfter, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(dbContextAfter), "public DbSet<PostTag> PostTags => Set<PostTag>();") != 1 {
		t.Fatalf("expected exactly one PostTag DbSet after re-running with --force: %s", dbContextAfter)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entities) != 3 {
		t.Fatalf("expected three recorded entities (Tag, Post, PostTag), got %#v", manifest.Entities)
	}
}

func TestEntityRelationshipGenerationDDD(t *testing.T) {
	project := filepath.Join(t.TempDir(), "DddRelDemo")
	if err := Run([]string{"new", "DddRelDemo", "--app", "fullstack", "--backend:ddd", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Customer", "name:string", "--backend:ddd", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Order", "total:decimal", "customer:Customer", "--backend:ddd", "--project", project}); err != nil {
		t.Fatal(err)
	}

	entity, err := os.ReadFile(filepath.Join(project, "src", "Domain", "Entities", "Order.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entity), "public long CustomerId { get; private set; }") || !strings.Contains(string(entity), "public Customer? Customer { get; private set; }") {
		t.Fatalf("expected foreign key and navigation property in Order.cs: %s", entity)
	}

	orderConfig, err := os.ReadFile(filepath.Join(project, "src", "Infrastructure", "Persistence", "Configurations", "OrderConfiguration.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(orderConfig), "builder.HasOne(x => x.Customer).WithMany().HasForeignKey(x => x.CustomerId);") {
		t.Fatalf("expected relation fluent config in OrderConfiguration.cs: %s", orderConfig)
	}

	customerConfig, err := os.ReadFile(filepath.Join(project, "src", "Infrastructure", "Persistence", "Configurations", "CustomerConfiguration.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(customerConfig), "HasOne") {
		t.Fatalf("Customer has no relations and should not get fluent config: %s", customerConfig)
	}

	dbContext, err := os.ReadFile(filepath.Join(project, "src", "Infrastructure", "Persistence", "AppDbContext.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dbContext), "modelBuilder.ApplyConfigurationsFromAssembly(typeof(AppDbContext).Assembly);") {
		t.Fatalf("expected ApplyConfigurationsFromAssembly in ddd AppDbContext.cs: %s", dbContext)
	}

	customerEntity, err := os.ReadFile(filepath.Join(project, "src", "Domain", "Entities", "Customer.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customerEntity), "public ICollection<Order> Orders { get; set; } = [];") {
		t.Fatalf("expected inverse Orders navigation in Customer.cs: %s", customerEntity)
	}
}

func TestEntityRelationshipGenerationRenoir(t *testing.T) {
	project := filepath.Join(t.TempDir(), "RenoirDemo")
	if err := Run([]string{"new", "RenoirDemo", "--app", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Customer", "name:string", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "customer:Customer", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}

	aggregate, err := os.ReadFile(filepath.Join(project, "src", "RenoirDemo.DomainModel", "Contexts", "Sales", "Aggregates", "Order.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregate), "public long CustomerId { get; private set; }") || !strings.Contains(string(aggregate), "public Customer? Customer { get; private set; }") {
		t.Fatalf("expected foreign key and navigation property in Order.cs: %s", aggregate)
	}

	config, err := os.ReadFile(filepath.Join(project, "src", "RenoirDemo.Persistence", "Contexts", "Sales", "OrderConfiguration.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "builder.HasOne(x => x.Customer).WithMany().HasForeignKey(x => x.CustomerId);") {
		t.Fatalf("expected relation fluent config in OrderConfiguration.cs: %s", config)
	}

	page, err := os.ReadFile(filepath.Join(project, "src", "RenoirDemo.AppBlazor", "Components", "Pages", "Sales", "OrderCrud.razor"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "@inject RenoirDemo.Application.Contexts.Sales.CustomerCrudService CustomerService") ||
		!strings.Contains(string(page), `<InputSelect @bind-Value="form.CustomerId" class="form-control">`) {
		t.Fatalf("expected a Customer picker InputSelect in OrderCrud.razor: %s", page)
	}

	customerAggregate, err := os.ReadFile(filepath.Join(project, "src", "RenoirDemo.DomainModel", "Contexts", "Sales", "Aggregates", "Customer.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customerAggregate), "public ICollection<Order> Orders { get; set; } = [];") {
		t.Fatalf("expected inverse Orders navigation in Customer.cs: %s", customerAggregate)
	}

	if err := Run([]string{"add", "context", "Support", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Ticket", "subject:string", "customer:Customer", "--context", "Support", "--project", project}); err == nil {
		t.Fatal("expected a cross-context relation to be rejected")
	}
	if _, err := os.Stat(filepath.Join(project, "src", "RenoirDemo.DomainModel", "Contexts", "Support", "Aggregates", "Ticket.cs")); !os.IsNotExist(err) {
		t.Fatalf("rejected cross-context aggregate generation left files behind: %v", err)
	}
}

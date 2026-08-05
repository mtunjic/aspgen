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

func TestManyToManyRelationGenerationRenoir(t *testing.T) {
	project := filepath.Join(t.TempDir(), "M2MRenoirDemo")
	if err := Run([]string{"new", "M2MRenoirDemo", "--app", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}

	joinAggregate, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.DomainModel", "Catalog", "PostTag.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(joinAggregate), "PostId") || !strings.Contains(string(joinAggregate), "TagId") {
		t.Fatalf("expected both foreign keys in the PostTag join aggregate: %s", joinAggregate)
	}

	crudService, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.Application", "PostTagCrudService.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(crudService), "PostId") || !strings.Contains(string(crudService), "TagId") {
		t.Fatalf("expected both foreign keys in PostTagCrudService.cs: %s", crudService)
	}

	postAggregate, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.DomainModel", "Catalog", "Post.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(postAggregate), "PostTag") {
		t.Fatalf("expected an inverse PostTag navigation on Post: %s", postAggregate)
	}

	tagAggregate, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.DomainModel", "Catalog", "Tag.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tagAggregate), "PostTag") {
		t.Fatalf("expected an inverse PostTag navigation on Tag: %s", tagAggregate)
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

	aggregate, err := os.ReadFile(filepath.Join(project, "src", "RenoirDemo.DomainModel", "Sales", "Order.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregate), "public long CustomerId { get; private set; }") || !strings.Contains(string(aggregate), "public Customer? Customer { get; private set; }") {
		t.Fatalf("expected foreign key and navigation property in Order.cs: %s", aggregate)
	}

	config, err := os.ReadFile(filepath.Join(project, "src", "RenoirDemo.Persistence", "OrderConfiguration.cs"))
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
	if !strings.Contains(string(page), "@inject RenoirDemo.Application.CustomerCrudService CustomerService") ||
		!strings.Contains(string(page), `<InputSelect @bind-Value="form.CustomerId" class="form-control">`) {
		t.Fatalf("expected a Customer picker InputSelect in OrderCrud.razor: %s", page)
	}

	customerAggregate, err := os.ReadFile(filepath.Join(project, "src", "RenoirDemo.DomainModel", "Sales", "Customer.cs"))
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
	if _, err := os.Stat(filepath.Join(project, "src", "RenoirDemo.DomainModel", "Support", "Ticket.cs")); !os.IsNotExist(err) {
		t.Fatalf("rejected cross-context aggregate generation left files behind: %v", err)
	}
}

// TestContextArchWpfUI covers -ui wpf on the --context/--arch engine: both
// attaching it at `new` time (aggregates added afterwards) and attaching it
// later via `add ui` (retrofitting a pre-existing aggregate).
func TestContextArchWpfUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "CqrsWpfDemo")
	if err := Run([]string{"new", "CqrsWpfDemo", "--context", "Billing", "--arch", "cqrs", "-ui", "wpf", "--output", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/Desktop/CqrsWpfDemo.Desktop.csproj")
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "--context", "Billing", "--project", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/Desktop/Modules/Product/Services/ProductStore.cs")
	assertExists(t, project, "src/Desktop/Modules/Product/Views/ProductView.xaml")

	store, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Product", "Services", "ProductStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(store), `"/api/billing/product"`) {
		t.Fatalf("expected a context-qualified route in ProductStore.cs: %s", store)
	}

	appHost, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "App.xaml.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appHost), "moduleCatalog.AddModule<CqrsWpfDemo.Desktop.Modules.Product.ProductModule>();") {
		t.Fatalf("expected ProductModule registered in App.xaml.cs: %s", appHost)
	}

	solution, err := os.ReadFile(filepath.Join(project, "CqrsWpfDemo.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(solution), "src\\Desktop\\CqrsWpfDemo.Desktop.csproj") {
		t.Fatalf("expected Desktop project in solution: %s", solution)
	}
	if !strings.Contains(string(solution), "CqrsWpfDemo.UnitTests") {
		t.Fatalf("expected test projects to survive the -ui wpf solution rewrite: %s", solution)
	}

	// retrofit case: es-tier project without -ui at `new` time, aggregate
	// added first, `add ui --framework wpf` attached afterwards.
	retrofit := filepath.Join(t.TempDir(), "EsWpfDemo")
	if err := Run([]string{"new", "EsWpfDemo", "--context", "Sales", "--arch", "es", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Order", "--framework", "wpf", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, retrofit, "src/Desktop/Modules/Order/Services/OrderStore.cs")
	retrofitStore, err := os.ReadFile(filepath.Join(retrofit, "src", "Desktop", "Modules", "Order", "Services", "OrderStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(retrofitStore), `"/api/sales/order"`) {
		t.Fatalf("expected a context-qualified route in retrofitted OrderStore.cs: %s", retrofitStore)
	}

	// dm tier now supports -ui wpf too, in-process (no WebApi host to call).
	dm := filepath.Join(t.TempDir(), "DmWpfDemo")
	if err := Run([]string{"new", "DmWpfDemo", "--context", "Ops", "--arch", "dm", "-ui", "wpf", "--output", dm}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Widget", "name:string", "--context", "Ops", "--project", dm}); err != nil {
		t.Fatal(err)
	}
	dmStore, err := os.ReadFile(filepath.Join(dm, "src", "Desktop", "Modules", "Widget", "Services", "WidgetStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dmStore), "WidgetCrudService service") {
		t.Fatalf("expected dm-tier WidgetStore.cs to call the CrudService in-process: %s", dmStore)
	}
	if strings.Contains(string(dmStore), "HttpClient") {
		t.Fatalf("dm-tier WidgetStore.cs should not use HttpClient (in-process only): %s", dmStore)
	}
	dmModule, err := os.ReadFile(filepath.Join(dm, "src", "Desktop", "Modules", "Widget", "WidgetModule.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dmModule), "containerRegistry.RegisterSingleton<DmWpfDemoDatabase>") ||
		!strings.Contains(string(dmModule), "containerRegistry.Register<IValidator<WidgetRequest>, WidgetValidator>();") ||
		!strings.Contains(string(dmModule), "containerRegistry.RegisterSingleton<WidgetCrudService>();") {
		t.Fatalf("expected dm-tier WidgetModule.cs to register the DbContext/validator/CrudService in DryIoc: %s", dmModule)
	}

	// retrofit case with a relation: aggregates added first, -ui wpf attached afterwards.
	dmRetrofit := filepath.Join(t.TempDir(), "DmWpfRetroDemo")
	if err := Run([]string{"new", "DmWpfRetroDemo", "--context", "Sales", "--arch", "dm", "--output", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Category", "name:string", "--context", "Sales", "--project", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "category:Category", "--context", "Sales", "--project", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Product", "--framework", "wpf", "--project", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, dmRetrofit, "src/Desktop/Modules/Category/Services/CategoryStore.cs")
	assertExists(t, dmRetrofit, "src/Desktop/Modules/Product/Services/ProductStore.cs")

	// ar tier is still rejected (no dm+ CrudService/aggregate support at all).
	ar := filepath.Join(t.TempDir(), "ArWpfDemo")
	if err := Run([]string{"new", "ArWpfDemo", "--context", "Ops", "--arch", "ar", "-ui", "wpf", "--output", ar}); err == nil {
		t.Fatal("expected -ui wpf to be rejected for an ar-tier context")
	}
}

// TestContextArchWpfUIMixedTiers covers a project whose contexts are at
// DIFFERENT arch tiers (cqrs bootstrapped first via `new`, dm added
// afterwards via `add context`) - a regression test for a bug where every
// aggregate's WPF Store used the SAME project-wide backend (whichever
// context was created first), instead of each aggregate's own context's
// tier, wrongly generating an HTTP Store (and a Desktop.csproj missing the
// Application project reference) for the dm-tier aggregate.
func TestContextArchWpfUIMixedTiers(t *testing.T) {
	project := filepath.Join(t.TempDir(), "MixedTierDemo")
	if err := Run([]string{"new", "MixedTierDemo", "--context", "Sales", "--arch", "cqrs", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "context", "Inventory", "--arch", "dm", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "StockItem", "quantity:int", "--context", "Inventory", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "wpf", "--framework", "wpf", "--project", project}); err != nil {
		t.Fatal(err)
	}

	stockItemStore, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "StockItem", "Services", "StockItemStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stockItemStore), "StockItemCrudService service") {
		t.Fatalf("expected the dm-tier StockItem aggregate to get an in-process Store even though Sales (cqrs) was created first: %s", stockItemStore)
	}
	if strings.Contains(string(stockItemStore), "HttpClient http") {
		t.Fatalf("dm-tier StockItemStore.cs should not use HttpClient: %s", stockItemStore)
	}

	orderStore, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Order", "Services", "OrderStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(orderStore), "HttpClient http") {
		t.Fatalf("expected the cqrs-tier Order aggregate to keep its HTTP Store: %s", orderStore)
	}

	desktopCsproj, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "MixedTierDemo.Desktop.csproj"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(desktopCsproj), `MixedTierDemo.Application\MixedTierDemo.Application.csproj`) {
		t.Fatalf("expected Desktop.csproj to reference the Application project since a dm-tier context exists: %s", desktopCsproj)
	}
}

// TestContextArchBlazorUI covers -ui blazor on the --context/--arch engine,
// mirroring TestContextArchWpfUI: attaching at `new` time (aggregates added
// afterwards) and attaching later via `add ui` (retrofitting a pre-existing
// aggregate).
func TestContextArchBlazorUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "BlazorCqrsDemo")
	if err := Run([]string{"new", "BlazorCqrsDemo", "--context", "Billing", "--arch", "cqrs", "-ui", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/BlazorCqrsDemo.AppBlazor.csproj")
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "--context", "Billing", "--project", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/Components/Pages/Billing/ProductCrud.razor")
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/Components/Pages/Billing/ProductDetails.razor")

	page, err := os.ReadFile(filepath.Join(project, "src", "BlazorCqrsDemo.AppBlazor", "Components", "Pages", "Billing", "ProductCrud.razor"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `private const string ApiPath = "/api/billing/product";`) {
		t.Fatalf("expected a context-qualified API path in ProductCrud.razor: %s", page)
	}

	solution, err := os.ReadFile(filepath.Join(project, "BlazorCqrsDemo.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(solution), "src\\BlazorCqrsDemo.AppBlazor\\BlazorCqrsDemo.AppBlazor.csproj") {
		t.Fatalf("expected AppBlazor project in solution: %s", solution)
	}
	if !strings.Contains(string(solution), "BlazorCqrsDemo.UnitTests") {
		t.Fatalf("expected test projects to survive the -ui blazor solution rewrite: %s", solution)
	}

	// retrofit case: es-tier project without -ui at `new` time, aggregate
	// added first, `add ui --framework blazor` attached afterwards.
	retrofit := filepath.Join(t.TempDir(), "EsBlazorDemo")
	if err := Run([]string{"new", "EsBlazorDemo", "--context", "Sales", "--arch", "es", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Order", "--framework", "blazor", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, retrofit, "src/EsBlazorDemo.AppBlazor/Components/Pages/Sales/OrderCrud.razor")
	retrofitPage, err := os.ReadFile(filepath.Join(retrofit, "src", "EsBlazorDemo.AppBlazor", "Components", "Pages", "Sales", "OrderCrud.razor"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(retrofitPage), `private const string ApiPath = "/api/sales/order";`) {
		t.Fatalf("expected a context-qualified API path in retrofitted OrderCrud.razor: %s", retrofitPage)
	}
}

// TestContextArchMvcUI covers -ui mvc on the --context/--arch engine: dm is
// the only supported tier (headless, in-process CrudService calls instead
// of HTTP), attaching both at `new` time and via `add ui` afterward
// (retrofitting pre-existing aggregates, including one with a relation).
func TestContextArchMvcUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "MvcDmDemo")
	if err := Run([]string{"new", "MvcDmDemo", "--context", "Billing", "--arch", "dm", "-ui", "mvc", "--output", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/MvcDmDemo.WebMvc/MvcDmDemo.WebMvc.csproj")
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "--context", "Billing", "--project", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/MvcDmDemo.WebMvc/Controllers/ProductController.cs")
	assertExists(t, project, "src/MvcDmDemo.WebMvc/Views/Product/Index.cshtml")

	controller, err := os.ReadFile(filepath.Join(project, "src", "MvcDmDemo.WebMvc", "Controllers", "ProductController.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(controller), `[Route("billing/product")]`) {
		t.Fatalf("expected a context-qualified route in ProductController.cs: %s", controller)
	}

	program, err := os.ReadFile(filepath.Join(project, "src", "MvcDmDemo.WebMvc", "Program.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(program), "builder.Services.AddScoped<ProductCrudService>();") {
		t.Fatalf("expected ProductCrudService registered in Program.cs: %s", program)
	}

	solution, err := os.ReadFile(filepath.Join(project, "MvcDmDemo.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(solution), "src\\MvcDmDemo.WebMvc\\MvcDmDemo.WebMvc.csproj") {
		t.Fatalf("expected WebMvc project in solution: %s", solution)
	}
	if !strings.Contains(string(solution), "MvcDmDemo.UnitTests") {
		t.Fatalf("expected test projects to survive the -ui mvc solution rewrite: %s", solution)
	}

	// retrofit case: dm-tier project without -ui at `new` time, two
	// aggregates (one relation) added first, `add ui --framework mvc`
	// attached afterwards - both must get retrofitted.
	retrofit := filepath.Join(t.TempDir(), "MvcRetroDemo")
	if err := Run([]string{"new", "MvcRetroDemo", "--context", "Sales", "--arch", "dm", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Category", "name:string", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "category:Category", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Product", "--framework", "mvc", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, retrofit, "src/MvcRetroDemo.WebMvc/Controllers/CategoryController.cs")
	assertExists(t, retrofit, "src/MvcRetroDemo.WebMvc/Controllers/ProductController.cs")

	// cqrs/es tiers still reject -ui mvc (no CrudService for es; wpf/blazor
	// cover cqrs already).
	cqrs := filepath.Join(t.TempDir(), "CqrsMvcDemo")
	if err := Run([]string{"new", "CqrsMvcDemo", "--context", "Ops", "--arch", "cqrs", "-ui", "mvc", "--output", cqrs}); err == nil {
		t.Fatal("expected -ui mvc to be rejected for a cqrs-tier context")
	}
}

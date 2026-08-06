package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertExists(t *testing.T, root, relative string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
		t.Errorf("expected generated file %s: %v", relative, err)
	}
}

func TestManyToManyRelationGenerationRenoir(t *testing.T) {
	project := filepath.Join(t.TempDir(), "M2MRenoirDemo")
	if err := Run([]string{"new", "M2MRenoirDemo", "--context", "Catalog", "--arch", "dm", "--output", project}); err != nil {
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

func TestEntityRelationshipGenerationRenoir(t *testing.T) {
	project := filepath.Join(t.TempDir(), "SalesDemo")
	if err := Run([]string{"new", "SalesDemo", "--context", "Sales", "--arch", "dm", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Customer", "name:string", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "customer:Customer", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}

	aggregate, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.DomainModel", "Sales", "Order.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregate), "public long CustomerId { get; private set; }") || !strings.Contains(string(aggregate), "public Customer? Customer { get; private set; }") {
		t.Fatalf("expected foreign key and navigation property in Order.cs: %s", aggregate)
	}

	config, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.Persistence", "OrderConfiguration.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "builder.HasOne(x => x.Customer).WithMany().HasForeignKey(x => x.CustomerId);") {
		t.Fatalf("expected relation fluent config in OrderConfiguration.cs: %s", config)
	}

	crudService, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.Application", "OrderCrudService.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(crudService), "CustomerId") {
		t.Fatalf("expected the Customer foreign key to appear in OrderCrudService.cs: %s", crudService)
	}

	customerAggregate, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.DomainModel", "Sales", "Customer.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customerAggregate), "public ICollection<Order> Orders { get; set; } = [];") {
		t.Fatalf("expected inverse Orders navigation in Customer.cs: %s", customerAggregate)
	}

	if err := Run([]string{"add", "context", "Support", "--arch", "dm", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Ticket", "subject:string", "customer:Customer", "--context", "Support", "--project", project}); err == nil {
		t.Fatal("expected a cross-context relation to be rejected")
	}
	if _, err := os.Stat(filepath.Join(project, "src", "SalesDemo.DomainModel", "Support", "Ticket.cs")); !os.IsNotExist(err) {
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
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/Components/Pages/Billing/ProductIndex.razor")
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/Components/Pages/Billing/ProductEdit.razor")
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/Components/Pages/Billing/ProductDetails.razor")

	page, err := os.ReadFile(filepath.Join(project, "src", "BlazorCqrsDemo.AppBlazor", "Components", "Pages", "Billing", "ProductIndex.razor"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `private const string ApiPath = "/api/billing/product";`) {
		t.Fatalf("expected a context-qualified API path in ProductIndex.razor: %s", page)
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
	assertExists(t, retrofit, "src/EsBlazorDemo.AppBlazor/Components/Pages/Sales/OrderIndex.razor")
	retrofitPage, err := os.ReadFile(filepath.Join(retrofit, "src", "EsBlazorDemo.AppBlazor", "Components", "Pages", "Sales", "OrderIndex.razor"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(retrofitPage), `private const string ApiPath = "/api/sales/order";`) {
		t.Fatalf("expected a context-qualified API path in retrofitted OrderIndex.razor: %s", retrofitPage)
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

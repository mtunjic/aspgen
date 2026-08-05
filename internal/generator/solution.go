package generator

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writeSolution(root, name, app string, force, simple bool, backend string, withTests bool, ui string) error {
	target := filepath.Join(root, name+".sln")
	if exists(target) && !force {
		return fmt.Errorf("refusing to overwrite %s", target)
	}
	projects := make([]string, 0, 8)
	targets := make([]string, 0, 8)
	if app == "webapi" || app == "fullstack" || (app == "wpf" && backend == "ddd") {
		projectsToAdd := []struct{ target, display, path string }{
			{"domain", name + ".Domain", "src\\Domain\\" + projectFileName(name, "Domain")},
			{"application", name + ".Application", "src\\Application\\" + projectFileName(name, "Application")},
			{"infrastructure", name + ".Infrastructure", "src\\Infrastructure\\" + projectFileName(name, "Infrastructure")},
			{"webapi", name + ".WebApi", "src\\WebApi\\" + projectFileName(name, "WebApi")},
		}
		if simple {
			projectsToAdd = []struct{ target, display, path string }{{"webapi", name + ".WebApi", "src\\WebApi\\" + projectFileName(name, "WebApi")}}
		}
		if app == "wpf" && backend == "ddd" {
			projectsToAdd = []struct{ target, display, path string }{
				{"domain", name + ".Domain", "src\\Domain\\" + projectFileName(name, "Domain")},
				{"application", name + ".Application", "src\\Application\\" + projectFileName(name, "Application")},
				{"infrastructure", name + ".Infrastructure", "src\\Infrastructure\\" + projectFileName(name, "Infrastructure")},
			}
		}
		for _, project := range projectsToAdd {
			projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", project.display, project.path, projectGUID(name, project.target)))
			targets = append(targets, project.target)
		}
	}
	// cqrs/es context-engine projects only get a Desktop project once -ui wpf
	// has been attached (dm has no WebApi host, but wpf works in-process there).
	if app == "wpf" || app == "fullstack" || ((app == "cqrs" || app == "es" || app == "dm") && ui == "wpf") {
		projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s.Desktop\", \"src\\Desktop\\%s.Desktop.csproj\", \"{%s}\"", name, name, projectGUID(name, "wpf")))
		targets = append(targets, "wpf")
	}
	// cqrs/es context-engine projects only get an AppBlazor project once
	// -ui blazor has been attached (dm has no WebApi host for it to call yet).
	if (app == "cqrs" || app == "es") && ui == "blazor" {
		projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", name+".AppBlazor", "src\\"+name+".AppBlazor\\"+name+".AppBlazor.csproj", projectGUID(name, "appblazor")))
		targets = append(targets, "appblazor")
	}
	if app == "blazor" {
		for _, project := range []struct{ target, display, path string }{
			{"domain", name + ".DomainModel", "src\\" + name + ".DomainModel\\" + name + ".DomainModel.csproj"},
			{"application", name + ".Application", "src\\" + name + ".Application\\" + name + ".Application.csproj"},
			{"infrastructure", name + ".Infrastructure", "src\\" + name + ".Infrastructure\\" + name + ".Infrastructure.csproj"},
			{"persistence", name + ".Persistence", "src\\" + name + ".Persistence\\" + name + ".Persistence.csproj"},
			{"resources", name + ".Resources", "src\\" + name + ".Resources\\" + name + ".Resources.csproj"},
			{"app", name + ".AppBlazor", "src\\" + name + ".AppBlazor\\" + name + ".AppBlazor.csproj"},
		} {
			projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", project.display, project.path, projectGUID(name, project.target)))
			targets = append(targets, project.target)
		}
	}
	// dm tier (--context/--arch engine): same class-library layers as blazor
	// minus AppBlazor, since no host is attached until `add ui` (Phase 5).
	if app == "dm" {
		for _, project := range []struct{ target, display, path string }{
			{"domain", name + ".DomainModel", "src\\" + name + ".DomainModel\\" + name + ".DomainModel.csproj"},
			{"application", name + ".Application", "src\\" + name + ".Application\\" + name + ".Application.csproj"},
			{"infrastructure", name + ".Infrastructure", "src\\" + name + ".Infrastructure\\" + name + ".Infrastructure.csproj"},
			{"persistence", name + ".Persistence", "src\\" + name + ".Persistence\\" + name + ".Persistence.csproj"},
			{"resources", name + ".Resources", "src\\" + name + ".Resources\\" + name + ".Resources.csproj"},
		} {
			projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", project.display, project.path, projectGUID(name, project.target)))
			targets = append(targets, project.target)
		}
	}
	// dm-tier context/arch projects only get a WebMvc project once -ui mvc
	// has been attached (dm's only UI option, in-process CrudService calls).
	if app == "dm" && ui == "mvc" {
		projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", name+".WebMvc", "src\\"+name+".WebMvc\\"+name+".WebMvc.csproj", projectGUID(name, "webmvc")))
		targets = append(targets, "webmvc")
	}
	// cqrs and es tiers (--context/--arch engine): dm tier's class-library
	// layers plus a headless-until-endpoints WebApi Minimal API host, since
	// vertical-slice features need somewhere to be mounted (unlike dm, which
	// stays headless).
	if app == "cqrs" || app == "es" {
		for _, project := range []struct{ target, display, path string }{
			{"domain", name + ".DomainModel", "src\\" + name + ".DomainModel\\" + name + ".DomainModel.csproj"},
			{"application", name + ".Application", "src\\" + name + ".Application\\" + name + ".Application.csproj"},
			{"infrastructure", name + ".Infrastructure", "src\\" + name + ".Infrastructure\\" + name + ".Infrastructure.csproj"},
			{"persistence", name + ".Persistence", "src\\" + name + ".Persistence\\" + name + ".Persistence.csproj"},
			{"resources", name + ".Resources", "src\\" + name + ".Resources\\" + name + ".Resources.csproj"},
			{"webapi", name + ".WebApi", "src\\WebApi\\" + projectFileName(name, "WebApi")},
		} {
			projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", project.display, project.path, projectGUID(name, project.target)))
			targets = append(targets, project.target)
		}
	}
	// Generated test projects (--context/--arch engine only, opt out with
	// --no-tests): every tier gets a UnitTests project exercising its
	// DbContext; dm has no WebApi host to test over HTTP, so it's the only
	// tier that skips IntegrationTests.
	if withTests {
		projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"",
			name+".UnitTests", "tests\\"+name+".UnitTests\\"+name+".UnitTests.csproj", projectGUID(name, "unittests")))
		targets = append(targets, "unittests")
		if app != "dm" {
			projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"",
				name+".IntegrationTests", "tests\\"+name+".IntegrationTests\\"+name+".IntegrationTests.csproj", projectGUID(name, "integrationtests")))
			targets = append(targets, "integrationtests")
		}
	}
	projectText := make([]string, 0, len(projects))
	for _, project := range projects {
		projectText = append(projectText, project+"\nEndProject")
	}
	config := make([]string, 0, len(projects)*4)
	for _, target := range targets {
		guid := projectGUID(name, target)
		config = append(config,
			fmt.Sprintf("\t\t{%s}.Debug|Any CPU.ActiveCfg = Debug|Any CPU", guid),
			fmt.Sprintf("\t\t{%s}.Debug|Any CPU.Build.0 = Debug|Any CPU", guid),
			fmt.Sprintf("\t\t{%s}.Release|Any CPU.ActiveCfg = Release|Any CPU", guid),
			fmt.Sprintf("\t\t{%s}.Release|Any CPU.Build.0 = Release|Any CPU", guid),
		)
	}
	content := "Microsoft Visual Studio Solution File, Format Version 12.00\n# Visual Studio Version 17\nVisualStudioVersion = 17.0.31903.59\nMinimumVisualStudioVersion = 10.0.40219.1\n" + strings.Join(projectText, "\n") + "\nGlobal\n\tGlobalSection(SolutionConfigurationPlatforms) = preSolution\n\t\tDebug|Any CPU = Debug|Any CPU\n\t\tRelease|Any CPU = Release|Any CPU\n\tEndGlobalSection\n\tGlobalSection(ProjectConfigurationPlatforms) = postSolution\n" + strings.Join(config, "\n") + "\n\tEndGlobalSection\nEndGlobal\n"
	return os.WriteFile(target, []byte(content), 0o644)
}

func projectGUID(project, target string) string {
	sum := sha1.Sum([]byte(project + ":" + target))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

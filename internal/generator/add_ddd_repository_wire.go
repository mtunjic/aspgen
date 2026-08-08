package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// wireCrudServiceToRepository patches an already-generated {{Aggregate}}CrudService
// so its mutating operations (Create/Update/Delete) go through the newly added
// repository instead of the raw DbContext, per DDD's Repository pattern
// ("delegate all object storage/access to the repositories"). Read operations
// (GetAll/GetById/Search) are left querying the DbContext directly since they
// project onto {{Aggregate}}View DTOs and the repository contract has no
// projection/search API — that split mirrors common CQRS practice and isn't a
// DDD violation. A no-op if the aggregate was generated with --no-crud.
func wireCrudServiceToRepository(r addRequest, m *Manifest, aggregateName, repositoryName string) error {
	crudPath := filepath.Join(r.Project, "src", m.Project+".Application", aggregateName+"CrudService.cs")
	if _, err := os.Stat(crudPath); err != nil {
		return nil
	}
	content, err := readPatchFile(crudPath)
	if err != nil {
		return err
	}

	oldCtor := fmt.Sprintf("public sealed class %sCrudService(IDbContextFactory<%sDatabase> databaseFactory, IValidator<%sRequest> validator)", aggregateName, m.Project, aggregateName)
	newCtor := fmt.Sprintf("public sealed class %sCrudService(IDbContextFactory<%sDatabase> databaseFactory, IValidator<%sRequest> validator, I%s repository)", aggregateName, m.Project, aggregateName, repositoryName)
	if content, err = replaceOnce(content, oldCtor, newCtor, "CrudService constructor"); err != nil {
		return err
	}

	oldCreate := "        entity.Init(\"system\");\n        database.Add(entity);\n        await database.SaveChangesSafelyAsync(cancellationToken);"
	newCreate := "        var result = await repository.AddAsync(entity, cancellationToken);\n        if (!result.Success) throw new InvalidOperationException(result.Message);"
	if content, err = replaceOnce(content, oldCreate, newCreate, "CrudService CreateAsync body"); err != nil {
		return err
	}

	notFound := fmt.Sprintf("if (entity is null) return CommandResponse.Fail().AddMessage(\"%s not found.\");", aggregateName)
	oldDelete := fmt.Sprintf("        var entity = await database.%ss.FirstOrDefaultAsync(x => x.Id == id && !x.Deleted, cancellationToken);\n        %s\n        entity.SoftDelete();\n        return await database.TrySaveChangesAsync(cancellationToken);", aggregateName, notFound)
	newDelete := "        return await repository.DeleteAsync(id, cancellationToken);"
	if content, err = replaceOnce(content, oldDelete, newDelete, "CrudService DeleteAsync body"); err != nil {
		return err
	}

	oldUpdateFetch := fmt.Sprintf("        var entity = await database.%ss.FirstOrDefaultAsync(x => x.Id == id && !x.Deleted, cancellationToken);\n        %s", aggregateName, notFound)
	newUpdateFetch := fmt.Sprintf("        var entity = await repository.GetByIdAsync(id, cancellationToken);\n        %s", notFound)
	if content, err = replaceOnce(content, oldUpdateFetch, newUpdateFetch, "CrudService UpdateAsync fetch"); err != nil {
		return err
	}

	oldUpdateSave := "        return await database.TrySaveChangesAsync(cancellationToken);"
	newUpdateSave := "        return await repository.SaveAsync(entity, cancellationToken);"
	if content, err = replaceOnce(content, oldUpdateSave, newUpdateSave, "CrudService UpdateAsync save"); err != nil {
		return err
	}

	// UpdateAsync returns the repository's CommandResponse; a failed Create
	// now throws so the UI never believes a record was saved when it was not.
	return writePatchFile(crudPath, content, r.DryRun)
}

// replaceOnce replaces the single expected occurrence of old with replacement,
// erroring if old is missing or appears more than once (an ambiguous match
// would silently corrupt the wrong spot).
func replaceOnce(content, old, replacement, description string) (string, error) {
	switch strings.Count(content, old) {
	case 0:
		return "", patchErr(description, old)
	case 1:
		return strings.Replace(content, old, replacement, 1), nil
	default:
		return "", fmt.Errorf("%s: expected pattern found more than once: %.80q", description, old)
	}
}

package golang

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/Jumpscale/go-raml/codegen/commons"
	"github.com/Jumpscale/go-raml/raml"
)

const (
	structTemplateLocation         = "./templates/struct.tmpl"
	inputValidatorTemplateLocation = "./templates/struct_input_validator.tmpl"
	inputValidatorFileResult       = "struct_input_validator.go"
)

// StructDef defines a struct
type structDef struct {
	T           raml.Type           // raml.Type of this struct
	Name        string              // struct's name
	Description []string            // structs description
	PackageName string              // package name
	Fields      map[string]fieldDef // all struct's fields
	OneLineDef  string              // not empty if this struct can be defined in one line
	Enum        *enum

	Validators []string
}

// true if this struct is not an alias of `interface{}`
func (sd structDef) NotBareInterface() bool {
	return !strings.HasSuffix(sd.OneLineDef, " interface{}")
}

// create new struct def
func newStructDef(name, packageName, description string, properties map[string]interface{}) structDef {
	// generate struct's fields from type properties
	fields := make(map[string]fieldDef)
	for k, v := range properties {
		prop := raml.ToProperty(k, v)
		fields[prop.Name] = newFieldDef(name, prop, packageName)
	}
	return structDef{
		Name:        name,
		PackageName: packageName,
		Fields:      fields,
		Description: commons.ParseDescription(description),
	}
}

// create struct definition from RAML Type node
func newStructDefFromType(t raml.Type, sName, packageName string) structDef {
	sd := newStructDef(sName, packageName, t.Description, t.Properties)
	sd.T = t

	// handle advanced type on raml1.0
	sd.handleAdvancedType()

	return sd
}

// create struct definition from RAML Body node
func newStructDefFromBody(body *raml.Bodies, structNamePrefix, packageName string, isGenerateRequest bool) structDef {
	// set struct name based on request or response
	structName := structNamePrefix + commons.RespBodySuffix
	if isGenerateRequest {
		structName = structNamePrefix + commons.ReqBodySuffix
	}

	// handle JSON string type
	// for example:
	// application/json: # media type
	//		type: | # structural definition of a response (schema or type)
	//			{
	//				"title": "Hello world Response",
	//				"type": "object",
	//					"properties": {
	//					"message": {
	//						"type": "string"
	//						}
	//					}
	//				}
	if body.ApplicationJSON.TypeString() != "" {
		var js raml.JSONSchema
		if err := json.Unmarshal([]byte(body.ApplicationJSON.TypeString()), &js); err == nil {
			return newStructDef(structName, packageName, js.Description, js.RAMLProperties())
		}
	}
	return newStructDef(structName, packageName, "", body.ApplicationJSON.Properties)
}

// generate Go struct
func (sd structDef) generate(dir string) error {
	// generate enums
	for _, f := range sd.Fields {
		if f.Enum != nil {
			if err := f.Enum.generate(dir); err != nil {
				return err
			}
		}
	}
	if sd.Enum != nil {
		return sd.Enum.generate(dir)
	}
	fileName := filepath.Join(dir, sd.Name+".go")
	return commons.GenerateFile(sd, structTemplateLocation, "struct_template", fileName, false)
}

// generate all structs from an RAML api definition
func generateStructs(types map[string]raml.Type, dir, packageName string) error {
	for name, t := range types {
		sd := newStructDefFromType(t, name, packageName)
		if err := sd.generate(dir); err != nil {
			return err
		}
	}
	return nil
}

// ImportPaths returns all packages that
// need to be imported by this struct
func (sd structDef) ImportPaths() map[string]struct{} {
	ip := map[string]struct{}{}

	if sd.needFmt() {
		ip["fmt"] = struct{}{}
	}
	if sd.OneLineDef == "" {
		ip["gopkg.in/validator.v2"] = struct{}{}
	}

	// libraries
	for _, fd := range sd.Fields {
		if fd.Type == "json.RawMessage" {
			ip["encoding/json"] = struct{}{}
		} else if lib := libImportPath(globRootImportPath, fd.Type); lib != "" {
			ip[lib] = struct{}{}
		}
	}
	return ip
}

// handle advance type type into structField
// example:
//   Mammal:
//     type: Animal
//     properties:
//       name:
//         type: string
// the additional fieldDef would be Animal composition
func (sd *structDef) handleAdvancedType() {
	if sd.T.Type == nil {
		sd.T.Type = "object"
	}

	strType := sd.T.TypeString()
	parents, isMultipleInherit := sd.T.MultipleInheritance()

	switch {
	case isMultipleInherit: //multiple inheritance
		sd.addMultipleInheritance(parents)
	case sd.T.IsUnion():
		sd.buildUnion()
	case sd.T.IsArray(): // arary type
		sd.buildArray()
	case strings.ToLower(strType) == "object": // plain type
		return
	case sd.T.IsEnum(): // enum
		sd.buildEnum()
	case strType != "" && len(sd.T.Properties) == 0: // type alias
		sd.buildTypeAlias()
	default: // single inheritance
		sd.addSingleInheritance(strType)
	}
}

// add single inheritance
// inheritance is implemented as composition
// spec : http://docs.raml.org/specs/1.0/#raml-10-spec-inheritance-and-specialization
func (sd *structDef) addSingleInheritance(strType string) {
	fd := fieldDef{
		Name:          strType,
		IsComposition: true,
	}
	sd.Fields[strType] = fd

}

// construct multiple inheritance to Go type
// example :
// Anggora:
//	 type: [ Animal , Cat ]
//	 properties:
//		color:
//			type: string
// The additional fielddef would be a composition of Animal & Cat
// http://docs.raml.org/specs/1.0/#raml-10-spec-multiple-inheritance
func (sd *structDef) addMultipleInheritance(parents []string) {
	for _, s := range parents {
		fieldType := strings.TrimSpace(s)
		fd := fieldDef{
			Name:          fieldType,
			IsComposition: true,
		}

		sd.Fields[fd.Name] = fd
	}
}

// buildEnum based on http://docs.raml.org/specs/1.0/#raml-10-spec-enums
// example result  `type TypeName []data_type`
func (sd *structDef) buildEnum() {
	sd.Enum = newEnumFromStruct(sd)
}

// build array type
// spec http://docs.raml.org/specs/1.0/#raml-10-spec-array-types
// example result  `type TypeName []something`
func (sd *structDef) buildArray() {
	sd.buildOneLine(convertToGoType(sd.T.Type.(string)))
}

// build union type
// union type is implemented as empty struct
func (sd *structDef) buildUnion() {
}

func (sd *structDef) buildTypeAlias() {
	sd.buildOneLine(convertToGoType(sd.T.Type.(string)))
}

func (sd *structDef) buildOneLine(tipe string) {
	sd.OneLineDef = "type " + sd.Name + " " + tipe
}

// generate input validator helper file
func generateInputValidator(packageName, dir string) error {
	var ctx = struct {
		PackageName string
	}{
		PackageName: packageName,
	}
	fileName := filepath.Join(dir, inputValidatorFileResult)
	return commons.GenerateFile(ctx, inputValidatorTemplateLocation, "struct_input_validator_template", fileName, true)
}

// true if this struct need to import 'fmt' package
// It is required by validation code,
// because validation error will need `fmt` to build error message
func (sd structDef) needFmt() bool {
	// array type min items and max items
	if sd.T.MinItems > 0 || sd.T.MaxItems > 0 {
		return true
	}

	// unique items
	for _, f := range sd.Fields {
		if f.UniqueItems {
			return true
		}
	}
	return false
}

func multipleInheritanceNewName(parents []string) string {
	return strings.Join(parents, "")
}

func unionNewName(tip string) string {
	tipes := strings.Split(tip, "|")
	for i, v := range tipes {
		tipes[i] = strings.TrimSpace(v)
	}
	return "Union" + strings.Join(tipes, "")
}

// create struct and generate it if possible.
// return:
// - newType Name if we try to generate it
// - nil if no error happened during generation
func createGenerateStruct(tip, dir, pkgName string) (string, error) {
	parents, isMultiple := commons.MultipleInheritance(tip)
	if isMultiple {
		sd := newStructDef(multipleInheritanceNewName(parents), pkgName, "", map[string]interface{}{})
		sd.addMultipleInheritance(parents)
		return sd.Name, sd.generate(dir)
	}
	if commons.IsUnion(tip) {
		t := raml.Type{
			Type: tip,
		}
		sd := newStructDef(unionNewName(tip), pkgName, "", map[string]interface{}{})
		sd.T = t
		sd.buildUnion()
		sd.generate(dir)
	}
	return "", nil
}

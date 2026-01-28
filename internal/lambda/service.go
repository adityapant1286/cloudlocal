package lambda

import (
	"archive/zip"
	"bytes"
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/utils"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func NewLambdaService() utils.ServiceHandler {
	cw := cloudwatch.GetServiceInstance()
	var lambdaEnabled = utils.IsServiceEnabled(LAMBDA)

	if lambdaEnabled {
		return &lambdaServiceImplementation{
			lambda: newLambdaService(cw),
			cw:     cw,
		}
	}
	return nil
}

func (svc *lambdaServiceImplementation) Handle(w http.ResponseWriter, r *http.Request, target string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		svc.cw.Error(SERVICE, "Dispatcher", fmt.Sprintf("Error reading request payload: %s", err.Error()))
		utils.RespondJSON(w, map[string]string{"message": err.Error()})
		return
	}

	userAgent := r.Header.Get("User-Agent")

	if strings.Contains(userAgent, "lambda.create-function") {

		functionConfig, err := svc.lambda.createFunction(body)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			utils.RespondJSON(w, map[string]interface{}{"message": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		utils.RespondJSON(w, functionConfig)
		return

	} else if strings.Contains(userAgent, "lambda.invoke") {
		path := r.URL.Path

		functionPath := strings.ReplaceAll(path, "/invocations", "")
		functionName := functionPath[strings.LastIndex(functionPath, "/")+1:]

		payload := string(body)

		req := InvokeRequest{
			FunctionName: functionName,
			Payload:      payload,
		}

		result, err := svc.lambda.invokeLocal(req)

		if err != nil {
			w.Header().Set("X-Amz-Function-Error", "Unhandled")
			w.WriteHeader(http.StatusInternalServerError)
			utils.RespondJSON(w, map[string]string{"message": err.Error()})
			return
		}

		// 4. Return the result as raw JSON bytes (as Lambda does)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var resp []byte

		if result == "" {
			resp = []byte("{}")
		} else {
			resp = []byte(result)
		}
		_, werr := w.Write(resp)
		if werr != nil {
			svc.cw.Error(SERVICE, functionName, fmt.Sprintf("Response error: %v", werr.Error()))
			return
		}
		return
	}

}

type internalLambda interface {
	createFunction(body []byte) (*FunctionConfig, error)
	invokeLocal(req InvokeRequest) (string, error)
}

func newLambdaService(cw cloudwatch.CwService) internalLambda {

	if err := os.MkdirAll(utils.LambdaDir, 0755); err != nil {
		cw.Error(SERVICE, "Config", fmt.Sprintf("Could not create lambda directory: %v", err))
	}

	funStateDir := filepath.Join(utils.LambdaDir, "state")
	path := filepath.Join(funStateDir, "function-state.json")

	if err := os.MkdirAll(funStateDir, 0755); err != nil {
		cw.Error(SERVICE, "Config", fmt.Sprintf("Could not create lambda state directory: %v", err))
	}

	svc := &lambdaSvcImplementation{
		cw:          cw,
		functions:   make(map[string]*FunctionConfig),
		storagePath: path,
	}

	svc.load()

	return svc
}

func (s *lambdaSvcImplementation) save() {
	state := persistentState{Functions: s.functions}
	data, _ := json.MarshalIndent(state, "", "  ")
	err := os.WriteFile(s.storagePath, data, 0644)

	if err != nil {
		s.cw.Error(SERVICE, "FunctionConfig", fmt.Sprintf("Error saving Lambda Function state: %s", err))
	}
}

func (s *lambdaSvcImplementation) load() {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return // File doesn't exist yet, which is fine
	}
	var state persistentState
	if err := utils.UnmarshalJsonErrors(data, &state); err == nil {
		s.functions = state.Functions
	}
}

func (s *lambdaSvcImplementation) createFunction(body []byte) (*FunctionConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req struct {
		FunctionName string            `json:"FunctionName"`
		Runtime      string            `json:"Runtime"`
		Role         string            `json:"Role"`
		Handler      string            `json:"Handler"`
		Code         map[string]string `json:"Code"`
	}
	utils.UnmarshalJson(body, &req)

	code, err := base64.StdEncoding.DecodeString(req.Code["ZipFile"])
	if err != nil {
		return nil, err
	}
	funcPath := filepath.Join(utils.LambdaDir, req.FunctionName)
	xtractErr := s.extractZip(code, funcPath)
	if xtractErr != nil {
		return nil, xtractErr
	}

	arn := fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", utils.AwsRegion, utils.AccountId, req.FunctionName)
	uuid := utils.RandomUuid()
	newFunctionCfg := &FunctionConfig{
		FunctionName: req.FunctionName,
		Runtime:      req.Runtime,
		Role:         req.Role,
		Handler:      req.Handler,
		RevisionId:   uuid,
		FunctionArn:  arn,
		LastModified: time.Now().Unix(),
	}

	s.functions[req.FunctionName] = newFunctionCfg
	s.save()
	s.cw.Info(SERVICE, "CreateFunction", fmt.Sprintf("Created Function config: %s", utils.MarshalIjson(newFunctionCfg)))
	return newFunctionCfg, nil
}

func (s *lambdaSvcImplementation) invokeLocal(req InvokeRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	functionConfig := s.functions[req.FunctionName]

	if functionConfig == nil {
		s.cw.Error(SERVICE, "Invoke", fmt.Sprintf("Function not found by name: %s", req.FunctionName))
		return "", fmt.Errorf("function not found by name: %s", req.FunctionName)
	}

	var cmd *exec.Cmd

	// Set a timeout to mimic Lambda behavior
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch functionConfig.Runtime {
	case "python":
		cmd = s.python(ctx, functionConfig, req)

	case "python3.14":
		cmd = s.python(ctx, functionConfig, req)

	case "nodejs":
		cmd = s.node(ctx, functionConfig, req)

	case "nodejs25.x":
		cmd = s.node(ctx, functionConfig, req)

	case "java21":
		cmd = s.java(ctx, functionConfig, req)

	case "java":
		cmd = s.java(ctx, functionConfig, req)

	default:
		return "", fmt.Errorf("runtime %s not supported yet", functionConfig.Runtime)
	}

	s.cw.Debug(SERVICE, "Invoke", fmt.Sprintf("Starting execution for %s", req.FunctionName))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = filepath.Join(utils.LambdaDir, functionConfig.FunctionName)

	err := cmd.Run()

	if err != nil {
		s.cw.Error(SERVICE, functionConfig.FunctionName, fmt.Sprintf("Err: %s, Stderr: %s", err.Error(), stderr.String()))
		return "", err
	}

	outstr := stdout.String()
	log.Printf("outstr: %s", outstr)
	s.cw.Info(SERVICE, functionConfig.FunctionName, fmt.Sprintf("%s", outstr))

	return outstr, nil
}

func (s *lambdaSvcImplementation) python(ctx context.Context, config *FunctionConfig, req InvokeRequest) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		"python3",
		"/opt/cloudlocal/shims/python_shim.py",
		config.Handler,
		req.Payload,
	)
}

func (s *lambdaSvcImplementation) node(ctx context.Context, config *FunctionConfig, req InvokeRequest) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		"node",
		"/opt/cloudlocal/shims/node_shim.js",
		config.Handler,
		req.Payload,
	)
}

func (s *lambdaSvcImplementation) java(ctx context.Context, config *FunctionConfig, req InvokeRequest) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		"java",
		"-jar",
		utils.LambdaDir+"/"+config.FunctionName+".jar",
		req.Payload,
	)
}

func (s *lambdaSvcImplementation) extractZip(zipData []byte, destFolder string) error {
	reader := bytes.NewReader(zipData)
	zipReader, err := zip.NewReader(reader, int64(len(zipData)))
	if err != nil {
		return err
	}

	for _, f := range zipReader.File {
		// 2. Build the destination path and prevent Zip Slip
		fpath := filepath.Join(destFolder, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(destFolder)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		// 3. Handle directories
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// 4. Create the parent directory for the file if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// 5. Open the file inside the ZIP and create the destination file
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		// Close both immediately to avoid resource leaks in large loops
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

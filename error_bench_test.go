package pipz_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func BenchmarkPipelineError_Error(b *testing.B) {
	baseErr := errors.New("processor failed")

	b.Run("SimpleError", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           baseErr,
			ProcessorName: "validator",
			StageIndex:    2,
			InputData:     42,
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})

	b.Run("ChainError", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           baseErr,
			ProcessorName: "validator",
			StageIndex:    2,
			InputData:     42,
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})

	b.Run("TimeoutError", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           baseErr,
			ProcessorName: "slow_processor",
			StageIndex:    3,
			InputData:     42,
			Timeout:       true,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Second * 5,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})

	b.Run("CanceledError", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           baseErr,
			ProcessorName: "canceled_processor",
			StageIndex:    1,
			InputData:     42,
			Timeout:       false,
			Canceled:      true,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 50,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})
}

func BenchmarkPipelineError_Unwrap(b *testing.B) {
	baseErr := errors.New("processor failed")
	pipelineErr := &pipz.PipelineError[int]{
		Err:           baseErr,
		ProcessorName: "validator",
		StageIndex:    2,
		InputData:     42,
		Timeout:       false,
		Canceled:      false,
		Timestamp:     time.Now(),
		Duration:      time.Millisecond * 100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pipelineErr.Unwrap() //nolint:errcheck
	}
}

func BenchmarkPipelineError_IsTimeout(b *testing.B) {
	b.Run("TimeoutTrue", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           errors.New("timeout"),
			ProcessorName: "slow_processor",
			StageIndex:    3,
			InputData:     42,
			Timeout:       true,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Second * 5,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.IsTimeout()
		}
	})

	b.Run("TimeoutFalse", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           errors.New("other error"),
			ProcessorName: "processor",
			StageIndex:    1,
			InputData:     42,
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.IsTimeout()
		}
	})

	b.Run("DeadlineExceeded", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           context.DeadlineExceeded,
			ProcessorName: "slow_processor",
			StageIndex:    3,
			InputData:     42,
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Second * 5,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.IsTimeout()
		}
	})
}

func BenchmarkPipelineError_IsCanceled(b *testing.B) {
	b.Run("CanceledTrue", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           errors.New("canceled"),
			ProcessorName: "canceled_processor",
			StageIndex:    1,
			InputData:     42,
			Timeout:       false,
			Canceled:      true,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 50,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.IsCanceled()
		}
	})

	b.Run("CanceledFalse", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           errors.New("other error"),
			ProcessorName: "processor",
			StageIndex:    1,
			InputData:     42,
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.IsCanceled()
		}
	})

	b.Run("ContextCanceled", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           context.Canceled,
			ProcessorName: "canceled_processor",
			StageIndex:    1,
			InputData:     42,
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 50,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.IsCanceled()
		}
	})
}

func BenchmarkPipelineError_DataTypes(b *testing.B) {
	baseErr := errors.New("processor failed")

	b.Run("IntData", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[int]{
			Err:           baseErr,
			ProcessorName: "validator",
			StageIndex:    2,
			InputData:     42,
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})

	b.Run("StringData", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[string]{
			Err:           baseErr,
			ProcessorName: "validator",
			StageIndex:    2,
			InputData:     "test data",
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})

	b.Run("StructData", func(b *testing.B) {
		type TestStruct struct {
			ID   int
			Name string
		}

		pipelineErr := &pipz.PipelineError[TestStruct]{
			Err:           baseErr,
			ProcessorName: "validator",
			StageIndex:    2,
			InputData:     TestStruct{ID: 1, Name: "test"},
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})

	b.Run("SliceData", func(b *testing.B) {
		pipelineErr := &pipz.PipelineError[[]int]{
			Err:           baseErr,
			ProcessorName: "validator",
			StageIndex:    2,
			InputData:     []int{1, 2, 3, 4, 5},
			Timeout:       false,
			Canceled:      false,
			Timestamp:     time.Now(),
			Duration:      time.Millisecond * 100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pipelineErr.Error()
		}
	})
}

func BenchmarkPipelineError_ErrorCreation(b *testing.B) {
	baseErr := errors.New("processor failed")

	b.Run("CreateError", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pipelineErr := &pipz.PipelineError[int]{
				Err:           baseErr,
				ProcessorName: fmt.Sprintf("processor_%d", i),
				StageIndex:    i % 10,
				InputData:     i,
				Timeout:       false,
				Canceled:      false,
				Timestamp:     time.Now(),
				Duration:      time.Millisecond * time.Duration(i%1000),
			}
			_ = pipelineErr
		}
	})

	b.Run("CreateAndStringify", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pipelineErr := &pipz.PipelineError[int]{
				Err:           baseErr,
				ProcessorName: fmt.Sprintf("processor_%d", i),
				StageIndex:    i % 10,
				InputData:     i,
				Timeout:       false,
				Canceled:      false,
				Timestamp:     time.Now(),
				Duration:      time.Millisecond * time.Duration(i%1000),
			}
			_ = pipelineErr.Error()
		}
	})
}

func BenchmarkPipelineError_ErrorChecking(b *testing.B) {
	baseErr := errors.New("processor failed")
	timeoutErr := &pipz.PipelineError[int]{
		Err:       context.DeadlineExceeded,
		Timeout:   true,
		Timestamp: time.Now(),
	}
	cancelErr := &pipz.PipelineError[int]{
		Err:       context.Canceled,
		Canceled:  true,
		Timestamp: time.Now(),
	}
	normalErr := &pipz.PipelineError[int]{
		Err:       baseErr,
		Timestamp: time.Now(),
	}

	b.Run("CheckTimeout", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = timeoutErr.IsTimeout()
		}
	})

	b.Run("CheckCanceled", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = cancelErr.IsCanceled()
		}
	})

	b.Run("CheckNormal", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = normalErr.IsTimeout()
			_ = normalErr.IsCanceled()
		}
	})
}

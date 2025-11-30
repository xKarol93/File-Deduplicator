//go:build gui

package main

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func runOnMain(fn func()) {
	if a := fyne.CurrentApp(); a != nil {
		drv := a.Driver()
		// try RunOnMain
		if r, ok := drv.(interface{ RunOnMain(func()) }); ok {
			r.RunOnMain(fn)
			return
		}
		if c, ok := drv.(interface{ CallOnMain(func()) }); ok {
			c.CallOnMain(fn)
			return
		}
	}
	go fn()
}

func main() {
	a := app.New()
	w := a.NewWindow("File Explorer - Duplicate finder (UI)")

	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("/path/to/scan")
	pathEntry.SetText("/path/to/scan")

	// binding for output so we can update it safely from goroutines
	outputBinding := binding.NewString()
	output := widget.NewMultiLineEntry()
	output.BaseWidget.ExtendBaseWidget(output)
	output.Bind(outputBinding)

	duplicatesBinding := binding.NewString()
	duplicates := widget.NewMultiLineEntry()
	duplicates.BaseWidget.ExtendBaseWidget(output)
	duplicates.Bind(duplicatesBinding)

	statusBinding := binding.NewString()
	statusBinding.Set("Ready")
	statusLabel := widget.NewLabelWithData(statusBinding)

	scanBtn := widget.NewButton("Scan", nil)
	scanBtn.Importance = widget.HighImportance

	hasRadio := widget.NewRadioGroup([]string{"MD5", "SHA1", "SHA256", "SHA512", "BLAKE3"}, func(selected string) {
	})
	hasRadio.Horizontal = true
	hasRadio.SetSelected("SHA256")

	hasCheck := widget.NewCheck("Write duplicates.json", nil)
	hasCheck.SetChecked(false)

	clearBtn := widget.NewButton("Clear output", nil)
	clearBtn.Disabled()

	deleteBtn := widget.NewButton("Remove Files", nil)
	deleteBtn.Importance = widget.DangerImportance
	deleteBtn.Disabled()

	scanBtn.OnTapped = func() {
		duplicateJSON := false
		dir := pathEntry.Text
		if dir == "" {
			statusBinding.Set("Set scan directory first")
			return
		}
		if hasCheck.Checked {
			duplicateJSON = true
		}
		selectedAlgo := hasRadio.Selected
		scanBtn.Disable()
		statusBinding.Set("Scanning...")
		outputBinding.Set("Starting scan...\n")

		lines := make(chan string, 256)
		progress := make(chan string, 256)
		start_time := time.Now()

		go ScanDirectory(dir, selectedAlgo, duplicateJSON, lines, progress)

		go func() {
			var accum strings.Builder
			var accumProgress strings.Builder
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			var buf strings.Builder
			var bufP strings.Builder

			flush := func(final bool) {
				if buf.Len() > 0 {
					accum.WriteString(buf.String())
					runOnMain(func() { _ = outputBinding.Set(accum.String()) })
					buf.Reset()
				}
				// flush progress/duplicates output
				if bufP.Len() > 0 {
					accumProgress.WriteString(bufP.String())
					runOnMain(func() { _ = duplicatesBinding.Set(accumProgress.String()) })
					bufP.Reset()
				}
				accum.WriteString(buf.String())
				runOnMain(func() { _ = outputBinding.Set(accum.String()) })
				buf.Reset()
				if final {
					finish_time := time.Since(start_time)
					statusBinding.Set(fmt.Sprintf("Done in %.2fs", finish_time.Seconds()))
					scanBtn.Enable()
				}
			}

			for {
				select {
				case l, ok := <-lines:
					if !ok {
						flush(true)
						return
					}
					buf.WriteString(l)
					if !strings.HasSuffix(l, "\n") {
						buf.WriteString("\n")
					}
				case p, ok := <-progress:
					if !ok {
						progress = nil
						if lines == nil && progress == nil {
							flush(true)
							return
						}
						continue
					}
					bufP.WriteString(p)
					if !strings.HasSuffix(p, "\n") {
						bufP.WriteString("\n")
					}
				case <-ticker.C:
					flush(false)
				}

			}
		}()
		clearBtn.Enable()
		temp, _ := duplicatesBinding.Get()
		if len(temp) > 0 {
			deleteBtn.Enable()
		}
		deleteBtn.Enable()
	}

	clearBtn.OnTapped = func() {
		outputBinding.Set("")
		duplicatesBinding.Set("")
		statusBinding.Set("Cleared")
	}

	deleteBtn.OnTapped = func() {
		outputBinding.Set("")
		start_time := time.Now()
		scanBtn.Disable()
		deleteBtn.Disable()
		clearBtn.Disable()
		statusBinding.Set("Removing files...")

		lines, err := duplicatesBinding.Get()
		if err != nil {
			statusBinding.Set("Error reading duplicates")
			return
		}
		toDelete := strings.Split(lines, "\n")
		deleted := 0
		for _, f := range toDelete {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			temp_binding, _ := outputBinding.Get()
			err = os.Remove(f)
			if err != nil {
				outputBinding.Set(temp_binding + fmt.Sprintf("File not found, skipping: %s\n", f))
				continue
			} else {
				deleted++
				outputBinding.Set(temp_binding + fmt.Sprintf("Deleted duplicate file: %s\n", f))
			}
		}
		duplicatesBinding.Set("")

		finish_time := time.Since(start_time)
		statusBinding.Set(fmt.Sprintf("Deleted %d files in %.2fs", deleted, finish_time.Seconds()))
		scanBtn.Enable()
		clearBtn.Enable()
	}

	dir_title := canvas.NewText("Dir to scan:", color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	dir_title.TextSize = 16
	dir_title.TextStyle.Bold = true
	hashing_title := canvas.NewText("Hashing algorithm:", color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	hashing_title.TextSize = 14
	hashing_title.TextStyle.Bold = true

	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(10, 10))
	hbox := container.NewHBox(scanBtn, spacer, clearBtn, spacer, deleteBtn)

	controls := container.NewVBox(
		dir_title,
		pathEntry,
		spacer,
		hashing_title,
		hasRadio,
		spacer,
		hasCheck,
		spacer,
		hbox,
		statusLabel,
	)
	log_title := canvas.NewText("Program Log:", color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	duplicates_title := canvas.NewText("Duplicates:", color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})

	outbox := container.NewBorder(log_title, nil, nil, nil, output)
	duplbox := container.NewBorder(duplicates_title, nil, nil, nil, duplicates)
	rightPane := container.NewVSplit(outbox, duplbox)
	rightPane.Offset = 0.5

	content := container.NewHSplit(controls, rightPane)
	content.Offset = 0.4

	w.SetContent(content)
	w.Resize(fyne.NewSize(1200, 800))
	w.ShowAndRun()
}

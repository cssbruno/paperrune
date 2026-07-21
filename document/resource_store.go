// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import "sort"

type resourceStore struct {
	fonts        map[string]fontDefinition
	templates    map[string]TemplateView
	images       map[string]*ImageInfo
	imageAliases map[string]string
	objects      resourceObjectNumbers
	attachments  attachmentResourceStore
}

type resourceObjectNumbers struct {
	templates map[string]int
}

type attachmentResourceStore struct {
	streams    map[attachmentStreamKey]int
	files      map[attachmentFileKey]int
	compressed map[attachmentStreamKey]attachmentStream
}

func newResourceStore() *resourceStore {
	return &resourceStore{
		fonts:        make(map[string]fontDefinition),
		templates:    make(map[string]TemplateView),
		images:       make(map[string]*ImageInfo),
		imageAliases: make(map[string]string),
		objects:      resourceObjectNumbers{templates: make(map[string]int)},
		attachments: attachmentResourceStore{
			streams:    make(map[attachmentStreamKey]int),
			files:      make(map[attachmentFileKey]int),
			compressed: make(map[attachmentStreamKey]attachmentStream),
		},
	}
}

func (state *resourceOwnershipState) initResourceStore() {
	state.resources = newResourceStore()
}

func (state *resourceOwnershipState) ensureResourceStore() *resourceStore {
	if state.resources == nil {
		state.resources = newResourceStore()
	}
	return state.resources
}

func (s *resourceStore) image(name string) (*ImageInfo, bool) {
	info, ok := s.images[name]
	if !ok && s.imageAliases != nil {
		info, ok = s.images[s.imageAliases[name]]
	}
	return info, ok
}

func (s *resourceStore) setImage(name string, info *ImageInfo) {
	s.images[name] = info
}

func (s *resourceStore) setImageAlias(alias, name string) {
	if alias == "" || name == "" || alias == name {
		return
	}
	if s.imageAliases == nil {
		s.imageAliases = make(map[string]string)
	}
	s.imageAliases[alias] = name
}

func (s *resourceStore) font(key string) (fontDefinition, bool) {
	font, ok := s.fonts[key]
	return font, ok
}

func (s *resourceStore) setFont(key string, font fontDefinition) {
	s.fonts[key] = font
}

func (s *resourceStore) fontsByResourceID(sorted bool) []fontDefinition {
	if !sorted {
		fonts := make([]fontDefinition, 0, len(s.fonts))
		for _, font := range s.fonts {
			fonts = append(fonts, font)
		}
		return fonts
	}
	keys := make([]string, 0, len(s.fonts))
	for key := range s.fonts {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left := s.fonts[keys[i]]
		right := s.fonts[keys[j]]
		if left.i != right.i {
			return left.i < right.i
		}
		return keys[i] < keys[j]
	})
	fonts := make([]fontDefinition, 0, len(keys))
	for _, key := range keys {
		fonts = append(fonts, s.fonts[key])
	}
	return fonts
}

func (s *resourceStore) fontsByKey(sorted bool) []fontDefinition {
	keys := make([]string, 0, len(s.fonts))
	for key := range s.fonts {
		keys = append(keys, key)
	}
	if sorted {
		sort.Strings(keys)
	}
	fonts := make([]fontDefinition, 0, len(keys))
	for _, key := range keys {
		fonts = append(fonts, s.fonts[key])
	}
	return fonts
}

func (s *resourceStore) imagesForOutput(sorted bool) []*ImageInfo {
	images := make([]*ImageInfo, 0, len(s.images))
	if !sorted {
		for _, image := range s.images {
			images = append(images, image)
		}
		return images
	}
	keys := make([]string, 0, len(s.images))
	for key := range s.images {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left := s.images[keys[i]]
		right := s.images[keys[j]]
		if left == nil || right == nil {
			return left != nil
		}
		if left.w != right.w {
			return left.w < right.w
		}
		if left.i != right.i {
			return left.i < right.i
		}
		return keys[i] < keys[j]
	})
	for _, key := range keys {
		images = append(images, s.images[key])
	}
	return images
}

func (s *resourceStore) imagesByResourceID(sorted bool) []*ImageInfo {
	if !sorted {
		images := make([]*ImageInfo, 0, len(s.images))
		for _, image := range s.images {
			images = append(images, image)
		}
		return images
	}
	keys := make([]string, 0, len(s.images))
	for key := range s.images {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left := s.images[keys[i]]
		right := s.images[keys[j]]
		if left == nil || right == nil {
			return left != nil
		}
		if left.i != right.i {
			return left.i < right.i
		}
		return keys[i] < keys[j]
	})
	images := make([]*ImageInfo, 0, len(keys))
	for _, key := range keys {
		images = append(images, s.images[key])
	}
	return images
}

func (s *resourceStore) addTemplate(tpl TemplateView) {
	s.templates[tpl.ID()] = tpl
}

func (s *resourceStore) templatesForOutput(sorted bool) []TemplateView {
	return sortTemplates(s.templates, sorted)
}

func (s *resourceStore) templateCatalogKeys(sorted bool) []string {
	return templateKeyList(s.templates, sorted)
}

func (s *resourceStore) template(id string) (TemplateView, bool) {
	tpl, ok := s.templates[id]
	return tpl, ok
}

func (s *resourceStore) templateObject(id string) (int, bool) {
	objID, ok := s.objects.templates[id]
	return objID, ok
}

func (s *resourceStore) setTemplateObject(id string, objID int) {
	s.objects.templates[id] = objID
}

func (s *resourceStore) templateOutputImage(tplID, name string, image *ImageInfo) *ImageInfo {
	if image == nil {
		return nil
	}
	if stored := s.images[sprintf("t%s-%s", tplID, name)]; stored != nil {
		return stored
	}
	keys := make([]string, 0, len(s.images))
	for key := range s.images {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		stored := s.images[key]
		if stored != nil && stored.i == image.i {
			return stored
		}
	}
	return image
}

func (s *resourceStore) compressedAttachment(key attachmentStreamKey) (attachmentStream, bool) {
	stream, ok := s.attachments.compressed[key]
	return stream, ok
}

func (s *resourceStore) addCompressedAttachment(key attachmentStreamKey, stream attachmentStream) bool {
	if _, ok := s.attachments.compressed[key]; ok {
		return false
	}
	s.attachments.compressed[key] = stream
	return true
}

func (s *resourceStore) attachmentStreamObject(key attachmentStreamKey) int {
	return s.attachments.streams[key]
}

func (s *resourceStore) setAttachmentStreamObject(key attachmentStreamKey, objectNumber int) {
	s.attachments.streams[key] = objectNumber
}

func (s *resourceStore) attachmentFileObject(key attachmentFileKey) int {
	return s.attachments.files[key]
}

func (s *resourceStore) setAttachmentFileObject(key attachmentFileKey, objectNumber int) {
	s.attachments.files[key] = objectNumber
}

func (s *resourceStore) cleanupAttachmentCompressedFiles() {
	for key, stream := range s.attachments.compressed {
		stream.cleanup()
		if stream.tempFile != "" {
			delete(s.attachments.compressed, key)
		}
	}
}

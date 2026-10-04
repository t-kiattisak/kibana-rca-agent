import { GoogleGenAI } from "@google/genai";
import * as dotenv from "dotenv";

dotenv.config({ path: "../.env" });

const apiKey = process.env.GEMINI_API_KEY || "";
export const geminiModel = process.env.GEMINI_MODEL || "gemini-3.5-flash";

export const ai = new GoogleGenAI({ apiKey });

export async function generateJSONWithRetry<T>(
  prompt: string,
  systemInstruction: string,
  schema: any,
  modelName: string = geminiModel
): Promise<{ data: T; promptTokens: number; responseTokens: number; totalTokens: number }> {
  let lastError: any = null;

  for (let attempt = 1; attempt <= 5; attempt++) {
    try {
      const response = await ai.models.generateContent({
        model: modelName,
        contents: prompt,
        config: {
          systemInstruction,
          responseMimeType: "application/json",
          responseSchema: schema,
          temperature: 0.2,
        },
      });

      const parsed = JSON.parse(response.text || "{}") as T;
      const usage = response.usageMetadata;

      return {
        data: parsed,
        promptTokens: usage?.promptTokenCount ?? 0,
        responseTokens: usage?.candidatesTokenCount ?? 0,
        totalTokens: usage?.totalTokenCount ?? 0,
      };
    } catch (err: any) {
      lastError = err;
      const errMsg = err?.message || String(err);
      if (errMsg.includes("503") || errMsg.includes("high demand") || errMsg.includes("UNAVAILABLE")) {
        console.warn(`[Gemini 503 Retry] Attempt ${attempt}/5: Backing off for ${attempt * 3}s...`);
        await new Promise((res) => setTimeout(res, attempt * 3000));
        continue;
      }
      throw err;
    }
  }

  throw lastError;
}

export async function generateEmbedding768(text: string): Promise<number[]> {
  const response = await ai.models.embedContent({
    model: "gemini-embedding-001",
    contents: text,
    config: {
      outputDimensionality: 768,
    },
  });

  return (response.embeddings?.[0]?.values as number[]) || [];
}
